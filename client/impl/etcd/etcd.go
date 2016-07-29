package etcd

import (
	"sync"
	"time"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/client"
	"github.com/contiv/volplugin/errors"
	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	etcdClient "github.com/coreos/etcd/client"
)

var existTranslation = map[client.ExistType]etcdClient.PrevExistType{
	client.PrevIgnore:  etcdClient.PrevIgnore,
	client.PrevExist:   etcdClient.PrevExist,
	client.PrevNoExist: etcdClient.PrevNoExist,
}

// Etcd is our abstraction of the etcd k/v store client.
type Etcd struct {
	client       etcdClient.KeysAPI
	pather       *client.Pather
	watchers     map[string][]chan struct{}
	watcherMutex sync.Mutex
}

// NewClient creates a new etcd client.
func NewClient(hosts []string, prefix string) (client.Client, error) {
	ec, err := etcdClient.New(etcdClient.Config{Endpoints: hosts})
	if err != nil {
		return nil, err
	}

	e := &Etcd{
		watchers: map[string][]chan struct{}{},
		client:   etcdClient.NewKeysAPI(ec),
		pather:   client.NewEtcdPather(prefix),
	}

	if _, err := e.client.Set(context.Background(), e.pather.String(), "", &etcdClient.SetOptions{Dir: true, PrevExist: etcdClient.PrevNoExist}); err != nil {
		if err != nil {
			er, ok := errors.EtcdToErrored(err).(*errored.Error)
			if !ok || !er.Contains(errors.Exists) {
				return nil, errored.New("Initial setup").Combine(err)
			}
		}
	}

	return e, nil
}

// Get retrieves a key from under the volplugin keyspace.
func (e *Etcd) Get(ctx context.Context, path *client.Pather, opts *client.GetOptions) (*client.Node, error) {
	etcdOpts := &etcdClient.GetOptions{Quorum: true}

	path, err := e.pather.Combine(path)
	if err != nil {
		return nil, err
	}

	resp, err := e.client.Get(ctx, path.String(), etcdOpts)
	if err != nil {
		return nil, errors.EtcdToErrored(err)
	}

	return e.toNode(resp.Node), nil
}

// Set publishes a key under the volplugin keyspace.
func (e *Etcd) Set(ctx context.Context, path *client.Pather, data []byte, opts *client.SetOptions) error {
	var etcdOpts *etcdClient.SetOptions

	if opts != nil {
		etcdOpts = &etcdClient.SetOptions{
			PrevExist: existTranslation[opts.Exist],
			PrevValue: string(opts.Value),
			Dir:       opts.Dir,
			TTL:       opts.TTL,
		}
	}

	path, err := e.pather.Combine(path)
	if err != nil {
		return err
	}

	_, err = e.client.Set(ctx, path.String(), string(data), etcdOpts)
	return errors.EtcdToErrored(err)
}

// Delete removes a key from the volplugin keyspace.
func (e *Etcd) Delete(ctx context.Context, path *client.Pather, opts *client.DeleteOptions) error {
	var etcdOpts *etcdClient.DeleteOptions

	if opts != nil {
		etcdOpts = &etcdClient.DeleteOptions{
			PrevValue: string(opts.Value),
			Recursive: opts.Recursive,
			Dir:       opts.Dir,
		}
	}

	path, err := e.pather.Combine(path)
	if err != nil {
		return err
	}

	_, err = e.client.Delete(ctx, path.String(), etcdOpts)
	return errors.EtcdToErrored(err)
}

func (e *Etcd) toNode(node *etcdClient.Node) *client.Node {
	retval := &client.Node{
		Dir:   node.Dir,
		Key:   node.Key,
		Value: []byte(node.Value),
		Nodes: []*client.Node{},
	}

	for _, node := range node.Nodes {
		retval.Nodes = append(retval.Nodes, e.toNode(node))
	}

	return retval
}

// Watch watches a keyspace and yields a func for processing the response. The
// response can be sent to a channel passed into your function which the
// reciever can listen on. This way you have the opportunity to translate it
// into a first-class object.
//
// The return value is a stop channel. Sending any item to it will immediately
// terminate the goroutine controlling the watch.
func (e *Etcd) Watch(path *client.Pather, channel chan interface{}, recursive bool, fun func(node *client.Node, channel chan interface{}) error) (chan struct{}, chan error) {
	e.watcherMutex.Lock()
	defer e.watcherMutex.Unlock()

	stopChan := make(chan struct{}, 1)
	errChan := make(chan error, 1)

	go func() {
		path, err := e.pather.Combine(path)
		if err != nil {
			errChan <- err
			return
		}

		watcher := e.client.Watcher(path.String(), &etcdClient.WatcherOptions{Recursive: recursive})

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-stopChan
			cancel()
		}()

		for {
			resp, err := watcher.Next(ctx)
			if err != nil {
				if err == context.Canceled {
					log.Debugf("watch for %q canceled", path)
					return
				}

				errChan <- err

				time.Sleep(time.Second)
				continue
			}

			if err := fun(e.toNode(resp.Node), channel); err != nil {
				errChan <- err
			}
		}
	}()

	strPath := path.String()

	_, ok := e.watchers[strPath]
	if !ok {
		e.watchers[strPath] = []chan struct{}{}
	}
	e.watchers[strPath] = append(e.watchers[strPath], stopChan)

	return stopChan, errChan
}

// WatchStop stops a watch. Given a path, stops all the watchers for a given path (f.e., it expired).
func (e *Etcd) WatchStop(path *client.Pather) {
	e.watcherMutex.Lock()
	defer e.watcherMutex.Unlock()

	strPath := path.String()

	if _, ok := e.watchers[strPath]; !ok {
		return
	}

	for _, sc := range e.watchers[strPath] {
		sc <- struct{}{}
	}

	delete(e.watchers, strPath)
}

// Path returns a pather object pointing at the root of the tree. Useful for
// escaping to the root in some deeper object hierarchies.
func (e *Etcd) Path() *client.Pather {
	return e.pather
}
