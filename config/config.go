package config

import (
	"encoding/json"
	"path"

	"github.com/contiv/go-etcd/etcd"
)

// RequestConfig provides a request structure for communicating with the
// volmaster.
type RequestConfig struct {
	Tenant string `json:"tenant"`
	Volume string `json:"volume"`
}

// RequestCreate provides a request structure for communicating with the
// volmaster, for create operations only.
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
	return &TopLevelConfig{
		Tenants:    map[string]*TenantConfig{},
		prefix:     prefix,
		etcdClient: etcd.NewClient(etcdHosts),
	}
}

func (c *TopLevelConfig) prefixed(str string) string {
	return path.Join(c.prefix, str)
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
