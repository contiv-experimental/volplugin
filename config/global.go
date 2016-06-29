package config

import (
	"encoding/json"
	"time"

	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/watch"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
)

const (
	// DefaultGlobalTTL is the TTL used when no TTL exists.
	DefaultGlobalTTL = 30 * time.Second
	// DefaultTimeout is the standard command timeout when none is provided.
	DefaultTimeout = 10 * time.Minute

	timeoutFixBase   = time.Minute
	ttlFixBase       = time.Second
	defaultMountPath = "/mnt/ceph"
)

// Global is the global configuration.
type Global struct {
	Debug     bool
	Timeout   time.Duration
	TTL       time.Duration
	MountPath string
}

// NewGlobalConfigFromJSON transforms json into a global.
func NewGlobalConfigFromJSON(content []byte) (*Global, error) {
	global := NewGlobalConfig()

	if err := json.Unmarshal(content, global); err != nil {
		return nil, err
	}

	global.fixupParameters()

	return global, nil
}

// NewGlobalConfig returns global config with preset defaults
func NewGlobalConfig() *Global {
	return &Global{
		TTL:       DefaultGlobalTTL,
		MountPath: defaultMountPath,
		Timeout:   DefaultTimeout,
	}
}

// PublishGlobal publishes the global configuration.
func (tlc *Client) PublishGlobal(g *Global) error {
	gcPath := tlc.prefixed("global-config")

	g.fixupParameters()
	value, err := json.Marshal(g)
	if err != nil {
		return err
	}

	if _, err := tlc.etcdClient.Set(context.Background(), gcPath, string(value), &client.SetOptions{PrevExist: client.PrevIgnore}); err != nil {
		return errors.EtcdToErrored(err)
	}

	return nil
}

// GetGlobal retrieves the global configuration.
func (tlc *Client) GetGlobal() (*Global, error) {
	resp, err := tlc.etcdClient.Get(context.Background(), tlc.prefixed("global-config"), nil)
	if err != nil {
		return nil, err
	}

	global := NewGlobalConfig()
	if err := json.Unmarshal([]byte(resp.Node.Value), global); err != nil {
		return nil, err
	}

	global.fixupParameters()

	return global, nil
}

func (global *Global) fixupParameters() {
	if global.Timeout == 0 {
		global.Timeout = DefaultTimeout
	}

	if global.TTL == 0 {
		global.TTL = DefaultGlobalTTL
	}

	if global.MountPath == "" {
		global.MountPath = defaultMountPath
	}

	if global.TTL < ttlFixBase {
		global.TTL = ttlFixBase * global.TTL
	}

	if global.Timeout < timeoutFixBase {
		global.Timeout = timeoutFixBase * global.Timeout
	}
}

// WatchGlobal watches a global and updates it as soon as the config changes.
func (tlc *Client) WatchGlobal(activity chan *watch.Watch) {
	w := watch.NewWatcher(activity, tlc.prefixed("global-config"), func(resp *client.Response, w *watch.Watcher) {
		global := &Global{}

		if resp.Action == "delete" {
			global = NewGlobalConfig()
		} else {
			if err := json.Unmarshal([]byte(resp.Node.Value), global); err != nil {
				log.Error("Error decoding global config, not updating")
				time.Sleep(time.Second)
				return
			}

			global.fixupParameters()
		}

		w.Channel <- &watch.Watch{Key: resp.Node.Key, Config: global}
	})

	watch.Create(w)
}
