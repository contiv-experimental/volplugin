package config

import (
	"encoding/json"
	"time"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
)

// DefaultGlobalTTL is the TTL used when no TTL exists.
const DefaultGlobalTTL = 30 * time.Second

var (
	timeoutFixBase = time.Minute
	ttlFixBase     = time.Second
)

// Global is the global configuration.
type Global struct {
	Debug   bool
	Timeout time.Duration
	TTL     time.Duration
}

// PublishGlobal publishes the global configuration.
func (tlc *TopLevelConfig) PublishGlobal(g *Global) error {
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
func (tlc *TopLevelConfig) GetGlobal() (*Global, error) {
	resp, err := tlc.etcdClient.Get(context.Background(), tlc.prefixed("global-config"), nil)
	if err != nil {
		return nil, err
	}

	global := &Global{}
	if err := json.Unmarshal([]byte(resp.Node.Value), global); err != nil {
		return nil, err
	}

	if global.TTL == 0 {
		global.TTL = DefaultGlobalTTL
	}

	return global, nil
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
func (tlc *TopLevelConfig) WatchGlobal(activity chan *Global) {
	watcher := tlc.etcdClient.Watcher(tlc.prefixed("global-config"), &client.WatcherOptions{Recursive: false})
	for {
		resp, err := watcher.Next(context.Background())
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		global := &Global{}
		if err := json.Unmarshal([]byte(resp.Node.Value), global); err != nil {
			log.Error("Error decoding global config, not updating")
			time.Sleep(1 * time.Second)
			continue
		}

		activity <- global
	}
}
