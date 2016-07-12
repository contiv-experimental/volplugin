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

	return global.SetEmpty(), nil
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

	value, err := json.Marshal(g.Canonical())
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

	return global.SetEmpty(), nil
}

// Published returns a copy of the current global with the parameters adjusted
// to fit the published representation. To see the internal/system/canonical
// version, please see Canonical() below.
//
// It is very important that you do not run this function multiple times
// against the same data set. It will adjust the parameters twice.
func (global *Global) Published() *Global {
	newGlobal := *global

	newGlobal.TTL /= ttlFixBase
	newGlobal.Timeout /= timeoutFixBase

	return &newGlobal
}

// Canonical returns a copy of the current global with the parameters adjusted
// to fit the internal (or canonical) representation. To see the published
// version, see Published() above.
//
// It is very important that you do not run this function multiple times
// against the same data set. It will adjust the parameters twice.
func (global *Global) Canonical() *Global {
	newGlobal := *global

	if global.TTL < ttlFixBase {
		newGlobal.TTL *= ttlFixBase
	}

	if global.Timeout < timeoutFixBase {
		newGlobal.Timeout *= timeoutFixBase
	}

	return &newGlobal
}

// SetEmpty sets any emptied parameters. This is typically used during the
// creation of the object accepted from user input.
func (global *Global) SetEmpty() *Global {
	newGlobal := *global

	if newGlobal.Timeout == 0 {
		newGlobal.Timeout = DefaultTimeout
	}

	if global.TTL == 0 {
		newGlobal.TTL = DefaultGlobalTTL
	}

	if global.MountPath == "" {
		newGlobal.MountPath = defaultMountPath
	}

	return &newGlobal
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
		}

		w.Channel <- &watch.Watch{Key: resp.Node.Key, Config: global}
	})

	watch.Create(w)
}
