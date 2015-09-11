package config

import (
	"encoding/json"
	"fmt"
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

// TenantConfig is the configuration of the tenant. It includes pool and
// snapshot information.
type TenantConfig struct {
	Pool         string         `json:"pool"`
	Size         uint64         `json:"size"`
	UseSnapshots bool           `json:"snapshots"`
	Snapshot     SnapshotConfig `json:"snapshot"`
}

// SnapshotConfig is the configuration for snapshots.
type SnapshotConfig struct {
	Frequency string `json:"frequency"`
	Keep      uint   `json:"keep"`
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

// PublishTenant publishes tenant intent to the configuration store.
func (c *TopLevelConfig) PublishTenant(name string, cfg *TenantConfig) error {
	if err := cfg.Validate(name); err != nil {
		return err
	}

	value, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	_, err = c.etcdClient.Set(c.prefixed(path.Join("tenants", name)), string(value), 0)
	return err
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

// Validate validates a tenant configuration, returning error on any issue.
func (cfg *TenantConfig) Validate(tenantName string) error {
	if cfg.Pool == "" {
		return fmt.Errorf("Config for tenant %q has a blank pool name", tenantName)
	}

	if cfg.Size == 0 {
		return fmt.Errorf("Config for tenant %q has a zero size", tenantName)
	}

	if cfg.UseSnapshots && (cfg.Snapshot.Frequency == "" || cfg.Snapshot.Keep == 0) {
		return fmt.Errorf("Snapshots are configured but cannot be used due to blank settings")
	}

	return nil
}
