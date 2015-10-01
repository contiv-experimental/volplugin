package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TenantConfig is the configuration of the tenant. It includes default
// information for items such as pool and volume configuration.
type TenantConfig struct {
	DefaultVolumeOptions VolumeOptions `json:"default-options"`
	DefaultPool          string        `json:"default-pool"`
}

func (c *TopLevelConfig) tenant(name string) string {
	return c.prefixed(rootTenant, name)
}

// PublishTenant publishes tenant intent to the configuration store.
func (c *TopLevelConfig) PublishTenant(name string, cfg *TenantConfig) error {
	if err := cfg.Validate(); err != nil {
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

	return nil
}

// DeleteTenant removes a tenant from the configuration store.
func (c *TopLevelConfig) DeleteTenant(name string) error {
	_, err := c.etcdClient.Delete(c.tenant(name), true)
	return err
}

// GetTenant retrieves a tenant from the configuration store.
func (c *TopLevelConfig) GetTenant(name string) (*TenantConfig, error) {
	resp, err := c.etcdClient.Get(c.tenant(name), true, false)
	if err != nil {
		return nil, err
	}

	tc := &TenantConfig{}
	err = json.Unmarshal([]byte(resp.Node.Value), tc)

	return tc, err
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

// Validate ensures the structure of the tenant is sane.
func (cfg *TenantConfig) Validate() error {
	if cfg.DefaultPool == "" {
		return fmt.Errorf("Default pool does not exist in new tenant")
	}

	return nil
}
