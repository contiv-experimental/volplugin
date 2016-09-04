package etcd

import (
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/impl/helpers"
	"github.com/contiv/volplugin/db/jsonio"
	"github.com/contiv/volplugin/errors"
	"github.com/coreos/etcd/client"
	wait "github.com/jbeda/go-wait"
	"golang.org/x/net/context"
)

// Client implements the db.Client interface.
type Client struct {
	prefix       string
	client       client.KeysAPI
	watchers     map[string]chan struct{}
	watcherMutex sync.Mutex
}

// NewClient creates a new Client with connections to etcd already established.
func NewClient(hosts []string, prefix string) (*Client, error) {
	ec, err := client.New(client.Config{Endpoints: hosts})
	if err != nil {
		return nil, err
	}

	c := &Client{
		client:   client.NewKeysAPI(ec),
		prefix:   prefix,
		watchers: map[string]chan struct{}{},
	}

	if _, err := c.client.Set(context.Background(), c.prefix, "", &client.SetOptions{Dir: true, PrevExist: client.PrevNoExist}); err != nil {
		if err != nil {
			er, ok := errors.EtcdToErrored(err).(*errored.Error)
			if !ok || !er.Contains(errors.Exists) {
				return nil, errored.New("Initial setup").Combine(err)
			}
		}
	}

	return c, nil
}

func (c *Client) qualified(path string) string {
	return strings.Join([]string{c.prefix, path}, "/")
}

// Get retrieves the item from etcd's key/value store and then populates obj with its data.
func (c *Client) Get(obj db.Entity) error {
	return helpers.WrapGet(c, obj, func(path string) (string, []byte, error) {
		resp, err := c.client.Get(context.Background(), c.qualified(path), nil)
		if err != nil {
			return "", nil, errors.EtcdToErrored(err)
		}

		return resp.Node.Key, []byte(resp.Node.Value), nil
	})
}

// Set takes the object and commits it to the database.
func (c *Client) Set(obj db.Entity) error {
	return helpers.WrapSet(c, obj, func(path string, content []byte) error {
		_, err := c.client.Set(context.Background(), c.qualified(path), string(content), nil)
		return err
	})
}

// Delete removes the object from the store.
func (c *Client) Delete(obj db.Entity) error {
	return helpers.WrapDelete(c, obj, func(path string) error {
		if _, err := c.client.Delete(context.Background(), c.qualified(path), nil); err != nil {
			return errors.EtcdToErrored(err)
		}

		return nil
	})
}

// Prefix returns a copy of the string used to make the database prefix.
func (c *Client) Prefix() string {
	return c.prefix
}

// Watch watches a given object for changes.
func (c *Client) Watch(obj db.Entity) (chan db.Entity, chan error) {
	path, err := obj.Path()
	if err != nil {
		errChan := make(chan error, 1)
		errChan <- err
		return nil, errChan
	}

	return helpers.WrapWatch(c, obj, path, false, c.watchers, &c.watcherMutex, c.startWatch)
}

// WatchStop stops a watch for a given object.
func (c *Client) WatchStop(obj db.Entity) error {
	path, err := obj.Path()
	if err != nil {
		return err
	}

	return helpers.WatchStop(c, path, c.watchers, &c.watcherMutex)
}

// WatchPrefix watches all items under the given entity's prefix
func (c *Client) WatchPrefix(obj db.Entity) (chan db.Entity, chan error) {
	return helpers.WrapWatch(c, obj, obj.Prefix(), true, c.watchers, &c.watcherMutex, c.startWatch)
}

// WatchPrefixStop stops
func (c *Client) WatchPrefixStop(obj db.Entity) error {
	return helpers.WatchStop(c, obj.Prefix(), c.watchers, &c.watcherMutex)
}

func (c *Client) startWatch(wi helpers.WatchInfo) {
	watcher := c.client.Watcher(c.qualified(wi.Path), &client.WatcherOptions{Recursive: wi.Recursive})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-wi.StopChan
		cancel()
	}()

	for {
		resp, err := watcher.Next(ctx)
		if err != nil {
			if err == context.Canceled {
				logrus.Debugf("watch for %q canceled", wi.Path)
				return
			}

			if _, ok := err.(*client.ClusterError); ok {
				logrus.Errorf("Received error during watch: %v -- aborting watch.", err)
				// silently return from the loop; this means the watch has died because
				// the server has died.
				return
			}

			wi.ErrorChan <- err

			time.Sleep(time.Second)
			continue
		}

		for _, entity := range c.traverse(resp.Node, wi.Object) {
			wi.ReturnChan <- entity
		}
	}
}

// traverse walks the keyspace and converts anything that looks like an entity
// into an entity and returns it as part of the array.
//
// traverse will log & skip errors to ensure bad data will not break this routine.
func (c *Client) traverse(node *client.Node, obj db.Entity) []db.Entity {
	entities := []db.Entity{}
	if node.Dir {
		for _, inner := range node.Nodes {
			entities = append(entities, c.traverse(inner, obj)...)
		}
	} else if copy, err := helpers.ReadAndSet(c, obj, node.Key, []byte(node.Value)); err == nil {
		entities = append(entities, copy)
	}

	return entities
}

// List populates obj with the list of the db in the collection
// corresponding to the entity. Will follow dirs until the ends of the earth
// trying to deserialize the entity.
func (c *Client) List(obj db.Entity) ([]db.Entity, error) {
	resp, err := c.client.Get(context.Background(), c.qualified(obj.Prefix()), &client.GetOptions{Recursive: true})
	if err != nil {
		return nil, err
	}

	return c.traverse(resp.Node, obj), nil
}

// ListPrefix is used to list a subtree of an entity, such as listing volume by policy.
func (c *Client) ListPrefix(prefix string, obj db.Entity) ([]db.Entity, error) {
	resp, err := c.client.Get(context.Background(), c.qualified(path.Join(obj.Prefix(), prefix)), &client.GetOptions{Recursive: true})
	if err != nil {
		return nil, err
	}

	return c.traverse(resp.Node, obj), nil
}

// Acquire and permanently hold a lock. Attempts until timeout. If timeout is
// zero, it will only try once.
func (c *Client) Acquire(lock db.Lock) error {
	logrus.Debugf("Acquiring lock %v", lock)

	if err := c.doAcquire(lock, 0); err != nil {
		return err
	}

	logrus.Debugf("Acquired lock %v", lock)

	return nil
}

func (c *Client) doAcquire(lock db.Lock, ttl time.Duration) error {
	content, err := jsonio.Write(lock)
	if err != nil {
		return errors.LockFailed.Combine(err)
	}

	path, err := lock.Path()
	if err != nil {
		return errors.LockFailed.Combine(err)
	}

	_, err = c.client.Set(context.Background(), c.qualified(path), string(content), &client.SetOptions{PrevValue: string(content), TTL: ttl})
	if er, ok := err.(client.Error); ok && er.Code == client.ErrorCodeKeyNotFound {
		_, err := c.client.Set(context.Background(), c.qualified(path), string(content), &client.SetOptions{PrevExist: client.PrevNoExist, TTL: ttl})
		if err != nil {
			return errors.LockFailed.Combine(err)
		}

		return nil
	}

	return err
}

// Free a lock. Pass force=true to force it dead.
func (c *Client) Free(lock db.Lock, force bool) error {
	content, err := jsonio.Write(lock)
	if err != nil {
		return errors.LockFailed.Combine(err)
	}

	path, err := lock.Path()
	if err != nil {
		return errors.LockFailed.Combine(err)
	}

	opts := &client.DeleteOptions{PrevValue: string(content)}

	if force {
		opts = &client.DeleteOptions{}
	}

	_, err = c.client.Delete(context.Background(), c.qualified(path), opts)
	return err
}

// AcquireAndRefresh starts a goroutine to refresh the key every 1/4
// (jittered) of the TTL. A stop channel is returned which, when sent a
// struct, will terminate the refresh. Error is returned for anything that
// might occur while setting up the goroutine.
//
// Do not use Free to free these locks, it will not work! Use the stop
// channel.
func (c *Client) AcquireAndRefresh(lock db.Lock, ttl time.Duration) (chan struct{}, error) {
	logrus.Debugf("In refresh, performing preliminary permanent lock on %v", lock)
	if err := c.Acquire(lock); err != nil {
		return nil, err
	}

	stopChan := make(chan struct{})

	go func() {
		for {
			select {
			case <-stopChan:
				if err := c.Free(lock, false); err != nil {
					logrus.Errorf("Error freeing lock %q: %v", lock, err)
				}

				return
			default:
				time.Sleep(wait.Jitter(ttl/4, 0))
				if err := c.AcquireWithTTL(lock, ttl); err != nil {
					logrus.Errorf("Could not acquire lock %v: %v", lock, err)
				}
			}
		}
	}()

	return stopChan, nil
}

// AcquireWithTTL acquires a lock with a TTL by using two compare-and-swap
// operations to refresh any lock that exists by us, and to take any lock that
// is not taken.
func (c *Client) AcquireWithTTL(lock db.Lock, ttl time.Duration) error {
	if ttl < 0 {
		err := errored.Errorf("TTL was less than 0 for locker %#v! This should not happen!", lock)
		logrus.Error(err)
		return err
	}

	logrus.Debugf("Acquiring lock %v with ttl %v", lock, ttl)

	if err := c.doAcquire(lock, ttl); err != nil {
		return err
	}

	logrus.Debugf("Acquired lock %v with ttl %v", lock, ttl)
	return nil
}

// Dump yields a database dump of the keyspace we manage. It will be contained
// in a tarball based on the timestamp of the dump. If a dir is provided, it
// will be placed under that directory.
func (c *Client) Dump(dir string) (string, error) {
	resp, err := c.client.Get(context.Background(), c.prefix, &client.GetOptions{Sort: true, Recursive: true, Quorum: true})
	if err != nil {
		return "", errored.Errorf(`Failed to recursively GET "%v" namespace from etcd`, c.prefix).Combine(err)
	}

	node := convertNode(resp.Node)

	return db.Dump(node, dir)
}

func convertNode(node *client.Node) *db.Node {
	ret := &db.Node{
		Key:   node.Key,
		Value: []byte(node.Value),
		Dir:   node.Dir,
		Nodes: []*db.Node{},
	}

	for _, inner := range node.Nodes {
		ret.Nodes = append(ret.Nodes, convertNode(inner))
	}

	return ret
}
