package config

import (
	"errors"
	"path"

	"github.com/contiv/go-etcd/etcd"
)

const (
	rootVolume = "volumes"
	rootMount  = "mounts"
	rootTenant = "tenants"
)

var (
	// ErrExist indicates when a key in etcd exits already. Used for create logic.
	ErrExist     = errors.New("Already exists")
	defaultPaths = []string{rootVolume, rootMount, rootTenant}
)

// Request provides a request structure for communicating with the
// volmaster.
type Request struct {
	Volume string `json:"volume"`
	Tenant string `json:"tenant"`
}

// RequestCreate provides a request structure for creating new volumes.
type RequestCreate struct {
	Tenant string            `json:"tenant"`
	Volume string            `json:"volume"`
	Opts   map[string]string `json:"opts"`
}

// TopLevelConfig is the top-level struct for communicating with the intent store.
type TopLevelConfig struct {
	etcdClient *etcd.Client
	prefix     string
}

// NewTopLevelConfig creates a TopLevelConfig struct which can drive communication
// with the configuration store.
func NewTopLevelConfig(prefix string, etcdHosts []string) *TopLevelConfig {
	config := &TopLevelConfig{
		prefix:     prefix,
		etcdClient: etcd.NewClient(etcdHosts),
	}

	config.etcdClient.SetDir(config.prefix, 0)
	for _, path := range defaultPaths {
		config.etcdClient.SetDir(config.prefixed(path), 0)
	}

	return config
}

func (c *TopLevelConfig) prefixed(strs ...string) string {
	str := c.prefix
	for _, s := range strs {
		str = path.Join(str, s)
	}

	return str
}
