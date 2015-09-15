package config

import (
	"encoding/json"
	"errors"
	"path"
	"strings"

	"github.com/contiv/go-etcd/etcd"
)

var (
	// ErrExist indicates when a key in etcd exits already. Used for create logic.
	ErrExist     = errors.New("Already exists")
	defaultPaths = []string{"tenants", "volumes", "mounts"}
)

// Request provides a request structure for communicating with the
// volmaster.
type Request struct {
	Volume string `json:"volume"`
	Pool   string `json:"pool"`
}

// RequestCreate provides a request structure for creating new volumes.
type RequestCreate struct {
	Tenant string `json:"tenant"`
	Volume string `json:"volume"`
	Pool   string `json:"pool"`
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

		tenantKey := strings.TrimPrefix(tenant.Key, c.prefixed("tenants"))

		c.Tenants[tenantKey] = cfg
	}

	return nil
}
