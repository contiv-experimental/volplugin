package config

import (
	"encoding/json"
	"time"

	"github.com/contiv/volplugin/storage/backend/ceph"
	"github.com/contiv/volplugin/watch"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
)

// DefaultGlobalTTL is the TTL used when no TTL exists.
const DefaultGlobalTTL = 30 * time.Second

var (
	timeoutFixBase   = time.Minute
	ttlFixBase       = time.Second
	defaultMountPath = "/mnt/ceph"
)

// Global is the global configuration.
type Global struct {
	Debug     bool
	Timeout   time.Duration
	TTL       time.Duration
	Backend   string
	MountPath string
}

// NewGlobalConfig returns global config with preset defaults
func NewGlobalConfig() *Global {
	return &Global{
		TTL:       DefaultGlobalTTL,
		Backend:   ceph.BackendName,
		MountPath: defaultMountPath,
	}
}

// PublishGlobal publishes the global configuration.
func (tlc *Client) PublishGlobal(g *Global) error {
	gcPath := tlc.prefixed("global-config")

	value, err := json.Marshal(MultiplyGlobalParameters(g))
	if err != nil {
		return err
	}

	if _, err := tlc.etcdClient.Set(context.Background(), gcPath, string(value), &client.SetOptions{PrevExist: client.PrevIgnore}); err != nil {
		return err
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
	if global.TTL == 0 {
		global.TTL = DefaultGlobalTTL
	}

	if global.MountPath == "" {
		global.MountPath = defaultMountPath
	}
}

// MultiplyGlobalParameters multiplies the paramters by a fixed base to allow them to
// be converted to proper time.Durations. This is to allow the global
// configuration to have small numbers of second or minutes, instead of giant
// numbers of nanoseconds in the JSON configuration.
// See also: DivideGlobalParameters.
func MultiplyGlobalParameters(global *Global) *Global {
	global2 := *global
	global2.TTL = ttlFixBase * global2.TTL
	global2.Timeout = timeoutFixBase * global2.Timeout
	return &global2
}

// DivideGlobalParameters does the inverse of MultiplyGlobalParameters.
func DivideGlobalParameters(global *Global) *Global {
	global2 := *global
	global2.TTL = global2.TTL / ttlFixBase
	global2.Timeout = global2.Timeout / timeoutFixBase
	return &global2
}

// WatchGlobal watches a global and updates it as soon as the config changes.
func (tlc *Client) WatchGlobal(activity chan *watch.Watch) {
	w := watch.NewWatcher(activity, tlc.prefixed("global-config"), func(resp *client.Response, w *watch.Watcher) {
		if resp.Action != "delete" {
			global := NewGlobalConfig()
			if err := json.Unmarshal([]byte(resp.Node.Value), global); err != nil {
				log.Error("Error decoding global config, not updating")
				time.Sleep(1 * time.Second)
				return
			}

			global.fixupParameters()

			w.Channel <- &watch.Watch{Key: resp.Node.Key, Config: global}
		}
	})

	watch.Create(w)
}
