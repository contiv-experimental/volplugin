package consul

import (
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/impl/helpers"
	"github.com/contiv/volplugin/errors"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/watch"
)

type lock struct {
	lock        *api.Lock
	sessionID   string
	doneChan    chan struct{}
	monitorChan <-chan struct{}
	obj         db.Lock
	ttl         time.Duration
	path        string
	refresh     bool
}

// Client implements the db.Client interface.
type Client struct {
	prefix string

	clientMutex sync.Mutex
	client      *api.Client
	config      *api.Config

	watchers     map[string]chan struct{}
	watcherMutex sync.Mutex

	lockMutex sync.Mutex
	locks     map[string]*lock

	clientResetSignal chan struct{}
}

// NewClient constructs a *Client for use. Requires a consul configuration and
// a keyspace prefix.
func NewClient(prefix string, config *api.Config) (*Client, error) {
	consulClient, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	if prefix != "" && prefix[0] == '/' {
		return nil, errored.Errorf("Consul paths may not start with / -- you supplied %q", prefix)
	}

	c := &Client{
		prefix:            prefix,
		client:            consulClient,
		watchers:          map[string]chan struct{}{},
		clientResetSignal: make(chan struct{}),
		config:            config,
		locks:             map[string]*lock{},
	}

	return c, nil
}

func (c *Client) qualified(path string) string {
	return strings.Join([]string{c.prefix, path}, "/")
}

// Prefix returns a copy of the string used to make the database prefix.
func (c *Client) Prefix() string {
	return c.prefix
}

// Set sets an object within consul. Returns an error on any problem.
func (c *Client) Set(obj db.Entity) error {
	return helpers.WrapSet(c, obj, func(path string, content []byte) error {
		_, err := c.client.KV().Put(&api.KVPair{Key: c.qualified(path), Value: content}, nil)
		return err
	})
}

// Get retrieves an object from consul, returns error on any problems.
func (c *Client) Get(obj db.Entity) error {
	return helpers.WrapGet(c, obj, func(path string) (string, []byte, error) {
		pair, _, err := c.client.KV().Get(c.qualified(path), nil)

		if err != nil {
			return "", nil, err
		}

		if pair == nil {
			return "", nil, errors.NotExists.Combine(errored.New(c.qualified(path)))
		}

		return pair.Key, pair.Value, nil
	})
}

// Delete removes a key from consul.
func (c *Client) Delete(obj db.Entity) error {
	return helpers.WrapDelete(c, obj, func(path string) error {
		_, err := c.client.KV().Delete(c.qualified(path), nil)
		return err
	})
}

// Watch watches stuff.
func (c *Client) Watch(obj db.Entity) (chan db.Entity, chan error) {
	path, err := obj.Path()
	if err != nil {
		errChan := make(chan error, 1)
		errChan <- err
		return nil, errChan
	}

	// these paths are qualified in watchInternal
	return helpers.WrapWatch(c, obj, path, false, c.watchers, &c.watcherMutex, c.watchInternal)
}

func (c *Client) watchInternal(wi helpers.WatchInfo) {
	var wp *watch.WatchPlan
	var err error

	if wi.Recursive {
		wp, err = watch.Parse(map[string]interface{}{
			"type":   "keyprefix",
			"prefix": c.qualified(wi.Path),
		})
	} else {
		wp, err = watch.Parse(map[string]interface{}{
			"type": "key",
			"key":  c.qualified(wi.Path),
		})
	}

	if err != nil {
		wi.ErrorChan <- err
		return
	}

	wp.Handler = func(u uint64, i interface{}) {
		if i == nil {
			return
		}

		switch i.(type) {
		case api.KVPairs:
			for _, pair := range i.(api.KVPairs) {
				if pair == nil {
					continue
				}

				this, err := helpers.ReadAndSet(c, wi.Object, pair.Key, pair.Value)
				if err != nil {
					wi.ErrorChan <- err
					continue
				}

				wi.ReturnChan <- this
			}
		case *api.KVPair:
			this, err := helpers.ReadAndSet(c, wi.Object, i.(*api.KVPair).Key, i.(*api.KVPair).Value)
			if err != nil {
				wi.ErrorChan <- err
				return
			}

			wi.ReturnChan <- this
		default:
			logrus.Errorf("received invalid pair %+v during watch", i)
		}
	}

	go func(wp *watch.WatchPlan) {
		if err := wp.Run(c.config.Address); err != nil {
			wi.ErrorChan <- err
		}
	}(wp)

	go func(wp *watch.WatchPlan, wi helpers.WatchInfo) {
		<-wi.StopChan
		(*wp).Stop()
	}(wp, wi)
}

// WatchPrefix watches a directory prefix for changes.
func (c *Client) WatchPrefix(obj db.Entity) (chan db.Entity, chan error) {
	// these paths are qualified in watchInternal
	return helpers.WrapWatch(c, obj, obj.Prefix(), true, c.watchers, &c.watcherMutex, c.watchInternal)
}

// WatchStop stops a watch.
func (c *Client) WatchStop(obj db.Entity) error {
	path, err := obj.Path()
	if err != nil {
		return err
	}

	return helpers.WatchStop(c, path, c.watchers, &c.watcherMutex)
}

// List takes an Entity which it will then populate an []Entity with a list of objects.
func (c *Client) List(obj db.Entity) ([]db.Entity, error) {
	pairs, _, err := c.client.KV().List(c.qualified(obj.Prefix()), nil)
	if err != nil {
		return nil, err
	}

	entities := []db.Entity{}

	for _, pair := range pairs {
		copy, err := helpers.ReadAndSet(c, obj, pair.Key, pair.Value)
		if err != nil {
			return nil, err
		}

		entities = append(entities, copy)
	}

	return entities, nil
}

// ListPrefix lists all the entities under prefix instead of listing the whole keyspace.
func (c *Client) ListPrefix(prefix string, obj db.Entity) ([]db.Entity, error) {
	pairs, _, err := c.client.KV().List(c.qualified(path.Join(obj.Prefix(), prefix)), nil)
	if err != nil {
		return nil, err
	}

	entities := []db.Entity{}

	for _, pair := range pairs {
		copy, err := helpers.ReadAndSet(c, obj, pair.Key, pair.Value)
		if err != nil {
			return nil, err
		}

		entities = append(entities, copy)
	}

	return entities, nil
}

// WatchPrefixStop stops a watch.
func (c *Client) WatchPrefixStop(obj db.Entity) error {
	return helpers.WatchStop(c, obj.Prefix(), c.watchers, &c.watcherMutex)
}

// Dump dumps a tarball made with mktemp() to the specified directory.
func (c *Client) Dump(dir string) (string, error) {
	pairs, _, err := c.client.KV().List(c.Prefix(), nil)
	if err != nil {
		return "", err
	}

	// consul uses a flat namespace so here be some hackery for directories and
	// recursive nodes.
	node := &db.Node{
		Key:   "/" + c.Prefix(), // dump routines expect the slash
		Dir:   true,
		Nodes: []*db.Node{},
	}

	for _, pair := range pairs {
		node.Nodes = append(node.Nodes, &db.Node{
			Key:   "/" + pair.Key,
			Value: pair.Value,
			Nodes: []*db.Node{},
		})
	}

	return db.Dump(node, dir)
}
