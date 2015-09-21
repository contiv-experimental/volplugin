package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

func (c *TopLevelConfig) tenant(name string) string {
	return c.prefixed(rootTenant, name)
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

	_, err = c.etcdClient.Set(c.tenant(name), string(value), 0)
	if err != nil {
		return err
	}

	c.Tenants[name] = cfg
	return nil
}

// DeleteTenant removes a tenant from the configuration store.
func (c *TopLevelConfig) DeleteTenant(name string) error {
	_, err := c.etcdClient.Delete(c.tenant(name), true)
	return err
}

// GetTenant retrieves a tenant from the configuration store.
func (c *TopLevelConfig) GetTenant(name string) (string, error) {
	resp, err := c.etcdClient.Get(c.tenant(name), true, false)
	if err != nil {
		return "", err
	}

	return resp.Node.Value, nil
}

// ListTenants provides an array of strings corresponding to the name of each
// tenant.
func (c *TopLevelConfig) ListTenants() ([]string, error) {
	resp, err := c.etcdClient.Get(c.prefixed(rootTenant), true, true)
	if err != nil {
		return nil, err
	}

	if resp.Node == nil {
		return nil, fmt.Errorf("Tenants root is missing")
	}

	tenants := []string{}

	for _, node := range resp.Node.Nodes {
		tenants = append(tenants, strings.TrimPrefix(node.Key, c.prefixed(rootTenant)))
	}

	return tenants, nil
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
