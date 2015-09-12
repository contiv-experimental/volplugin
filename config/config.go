package config

import (
	"encoding/json"
	"errors"
	"path"

	"github.com/contiv/go-etcd/etcd"
)

var (
	// ErrExist indicates when a key in etcd exits already. Used for create logic.
	ErrExist = errors.New("Already exists")
)

// Request provides a request structure for communicating with the
// volmaster.
type Request struct {
	Volume string `json:"volume"`
}

// RequestCreate provides a request structure for creating new volumes.
type RequestCreate struct {
	Tenant string `json:"tenant"`
	Volume string `json:"volume"`
}

// TopLevelConfig is the top-level struct for communicating with the intent store.
type TopLevelConfig struct {
	Tenants map[string]*TenantConfig

	etcdClient *etcd.Client
	prefix     string
}

// NewTopLevelConfig creates a TopLevelConfig struct which can drive communication
// with the configuration store.
func NewTopLevelConfig(prefix string, etcdHosts []string) *TopLevelConfig {
	config := &TopLevelConfig{
		Tenants:    map[string]*TenantConfig{},
		prefix:     prefix,
		etcdClient: etcd.NewClient(etcdHosts),
	}

	config.etcdClient.SetDir(config.prefix, 0)
	config.etcdClient.SetDir(config.prefixed("tenants"), 0)
	config.etcdClient.SetDir(config.prefixed("volumes"), 0)

	return config
}

func (c *TopLevelConfig) prefixed(strs ...string) string {
	str := c.prefix
	for _, s := range strs {
		str = path.Join(str, s)
	}

	return str
}

// Sync populates all tenants from the configuration store.
func (c *TopLevelConfig) Sync() error {
	resp, err := c.etcdClient.Get(c.prefixed("tenants"), true, true)
	if err != nil {
		return err
	}

	for _, tenant := range resp.Node.Nodes {
		cfg := &TenantConfig{}
		if err := json.Unmarshal([]byte(tenant.Value), cfg); err != nil {
			return err
		}

		if err := cfg.Validate(tenant.Key); err != nil {
			return err
		}

		c.Tenants[path.Base(tenant.Key)] = cfg
	}

	return nil
}
