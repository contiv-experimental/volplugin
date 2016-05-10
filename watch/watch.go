// Package watch is a singleton registry of path -> etcd watches. It does the
// watching in a uniform manner, and uses provided functions to perform the
// specific work.
package watch

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// Watch is a struct providing a generic way of passing key/value information
// from etcd that is already unmarshalled.
type Watch struct {
	Key    string
	Config interface{}
}

// WatcherFunc is the function that's fired anytime the watch receives an event
// that is not an error. It is expected that the WatcherFunc send to the
// channel on success.
type WatcherFunc func(*client.Response, *Watcher)

// Watcher is a struct that describes the various channels and data that
// comprise a successful watch on etcd.
//
// Note in particular the Channel, which is an interface{}. The channel must be
// of type `chan` and it is expected that a listener exists for that channel
// elsewhere.
type Watcher struct {
	WatcherFunc
	Path         string
	Channel      chan *Watch
	StopChannel  chan struct{}
	ErrorChannel chan error
	StopOnError  bool
	Recursive    bool
}

var etcdClient client.KeysAPI

var (
	watchers     = map[string][]*Watcher{}
	watcherMutex = sync.Mutex{}
)

// Init must be called before any watches can be established. Any attempt to
// create a watch before this is called will not yield an error but just log
// instead, without creating the watch. This should always be called before
// you're ready to use watches.
func Init(ec client.KeysAPI) {
	etcdClient = ec
}

// NewWatcher creates a basic Watcher with the channel, path, and WatcherFunc.
// NewWatcher will create the other channels and set recursive.
func NewWatcher(channel chan *Watch, path string, fun WatcherFunc) *Watcher {
	return &Watcher{
		WatcherFunc:  fun,
		Channel:      channel,
		Path:         path,
		StopChannel:  make(chan struct{}, 1),
		ErrorChannel: make(chan error, 1),
		Recursive:    true,
	}
}

// Create a watch. Given a watcher, creates a watch and runs it in a goroutine,
// then registers it with the watch registry.
func Create(w *Watcher) {
	if etcdClient == nil {
		log.Error("etcdClient is nil, cannot watch anything!")
		return
	}

	watcherMutex.Lock()
	defer watcherMutex.Unlock()

	go func(w *Watcher) {
		watcher := etcdClient.Watcher(w.Path, &client.WatcherOptions{Recursive: w.Recursive})

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-w.StopChannel
			cancel()
		}()

		for {
			resp, err := watcher.Next(ctx)
			if err != nil {
				if err == context.Canceled {
					log.Debugf("watch for %q canceled", w.Path)
					return
				}

				w.ErrorChannel <- err
				if w.StopOnError {
					w.StopChannel <- struct{}{}
					return
				}

				time.Sleep(1 * time.Second)
				continue
			}

			w.WatcherFunc(resp, w)
		}
	}(w)

	wary, ok := watchers[w.Path]
	if !ok {
		wary = []*Watcher{}
		watchers[w.Path] = wary
	}
	watchers[w.Path] = append(watchers[w.Path], w)
}

// Stop a watch. Given a path, stops all the watchers for a given path (f.e., it expired).
func Stop(path string) {
	watcherMutex.Lock()
	defer watcherMutex.Unlock()

	if _, ok := watchers[path]; !ok {
		return
	}

	for _, w := range watchers[path] {
		w.StopChannel <- struct{}{}
	}

	delete(watchers, path)
}
