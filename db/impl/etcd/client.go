package etcd

import (
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/jsonio"
	"github.com/contiv/volplugin/errors"
	"github.com/coreos/etcd/client"
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
	if obj.Hooks().PreGet != nil {
		if err := obj.Hooks().PreGet(c, obj); err != nil {
			return errors.EtcdToErrored(err)
		}
	}

	path, err := obj.Path()
	if err != nil {
		return err
	}

	resp, err := c.client.Get(context.Background(), c.qualified(path), nil)
	if err != nil {
		return errors.EtcdToErrored(err)
	}

	if err := jsonio.Read(obj, []byte(resp.Node.Value)); err != nil {
		return err
	}

	if err := obj.SetKey(c.trimPath(resp.Node.Key)); err != nil {
		return err
	}

	if obj.Hooks().PostGet != nil {
		if err := obj.Hooks().PostGet(c, obj); err != nil {
			return errors.EtcdToErrored(err)
		}
	}

	return obj.Validate()
}

// Set takes the object and commits it to the database.
func (c *Client) Set(obj db.Entity) error {
	if err := obj.Validate(); err != nil {
		return err
	}

	if obj.Hooks().PreSet != nil {
		if err := obj.Hooks().PreSet(c, obj); err != nil {
			return errors.EtcdToErrored(err)
		}
	}

	content, err := jsonio.Write(obj)
	if err != nil {
		return err
	}

	path, err := obj.Path()
	if err != nil {
		return err
	}

	if _, err := c.client.Set(context.Background(), c.qualified(path), string(content), nil); err != nil {
		return errors.EtcdToErrored(err)
	}

	if obj.Hooks().PostSet != nil {
		if err := obj.Hooks().PostSet(c, obj); err != nil {
			return errors.EtcdToErrored(err)
		}
	}

	return nil
}

// Delete removes the object from the store.
func (c *Client) Delete(obj db.Entity) error {
	if obj.Hooks().PreDelete != nil {
		if err := obj.Hooks().PreDelete(c, obj); err != nil {
			return errors.EtcdToErrored(err)
		}
	}

	path, err := obj.Path()
	if err != nil {
		return err
	}

	if _, err := c.client.Delete(context.Background(), c.qualified(path), nil); err != nil {
		return errors.EtcdToErrored(err)
	}

	if obj.Hooks().PostDelete != nil {
		if err := obj.Hooks().PostDelete(c, obj); err != nil {
			return errors.EtcdToErrored(err)
		}
	}

	return nil
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
		return make(chan db.Entity), errChan
	}

	return c.watchPath(obj, path, false)
}

// WatchStop stops a watch for a given object.
func (c *Client) WatchStop(obj db.Entity) error {
	path, err := obj.Path()
	if err != nil {
		return err
	}

	return c.watchStopPath(path)
}

// WatchPrefix watches all items under the given entity's
func (c *Client) WatchPrefix(obj db.Entity) (chan db.Entity, chan error) {
	return c.watchPath(obj, obj.Prefix(), true)
}

// WatchPrefixStop stops
func (c *Client) WatchPrefixStop(obj db.Entity) error {
	return c.watchStopPath(obj.Prefix())
}

// WatchPath watches an object for changes. Returns two channels: one for entity updates and one for errors.
// You can stop watches with WatchStop with the same path.
// Only one watch for a given path may be active at a time.
func (c *Client) watchPath(obj db.Entity, path string, recursive bool) (chan db.Entity, chan error) {
	c.watcherMutex.Lock()
	defer c.watcherMutex.Unlock()

	stopChan := make(chan struct{}, 1)
	retChan := make(chan db.Entity)
	errChan := make(chan error, 1)

	go func() {
		watcher := c.client.Watcher(c.qualified(path), &client.WatcherOptions{Recursive: recursive})

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-stopChan
			cancel()
		}()

		for {
			resp, err := watcher.Next(ctx)
			if err != nil {
				if err == context.Canceled {
					logrus.Debugf("watch for %q canceled", path)
					return
				}

				errChan <- err

				time.Sleep(time.Second)
				continue
			}

			for _, entity := range c.traverse(resp.Node, obj) {
				retChan <- entity
			}
		}
	}()

	_, ok := c.watchers[path]
	if ok {
		close(c.watchers[path])
	}
	c.watchers[path] = stopChan

	return retChan, errChan
}

// WatchStopPath stops a watch given a path to stop the watch on.
func (c *Client) watchStopPath(path string) error {
	c.watcherMutex.Lock()
	defer c.watcherMutex.Unlock()

	stopChan, ok := c.watchers[path]
	if !ok {
		return errors.InvalidDBPath.Combine(errored.New("missing key during watch"))
	}

	close(stopChan)
	delete(c.watchers, path)

	return nil
}

func (c *Client) trimPath(key string) string {
	return strings.Trim(strings.TrimPrefix(strings.Trim(key, "/"), c.Prefix()), "/")
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
	} else {
		copy := obj.Copy()

		doAppend := true

		if err := jsonio.Read(copy, []byte(node.Value)); err != nil {
			// This is kept this way so a buggy policy won't break listing all of them
			logrus.Errorf("Recieved error retrieving value at path %q during list: %v", node.Key, err)
			doAppend = false
		}

		if err := copy.SetKey(c.trimPath(node.Key)); err != nil {
			logrus.Error(err)
			doAppend = false
		}

		// same here. fire hooks to retrieve the full entity. only log but don't append on error.
		if copy.Hooks().PostGet != nil {
			if err := copy.Hooks().PostGet(c, copy); err != nil {
				logrus.Errorf("Error received trying to run fetch hooks during %q list: %v", node.Key, err)
				doAppend = false
			}
		}

		if doAppend {
			entities = append(entities, copy)
		}
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
