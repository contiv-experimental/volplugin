package config

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/coreos/etcd/client"

	"golang.org/x/net/context"
)

// TenantConfig is the configuration of the tenant. It includes default
// information for items such as pool and volume configuration.
type TenantConfig struct {
	DefaultVolumeOptions VolumeOptions     `json:"default-options"`
	FileSystems          map[string]string `json:"filesystems"`
}

var defaultFilesystems = map[string]string{
	"ext4": "mkfs.ext4 -m0 %",
}

const defaultFilesystem = "ext4"

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

	c.etcdClient.Set(context.Background(), c.prefixed(rootTenant, name), "", &client.SetOptions{Dir: true})

	if _, err := c.etcdClient.Set(context.Background(), c.tenant(name), string(value), &client.SetOptions{PrevExist: client.PrevIgnore}); err != nil {
		return err
	}

	return nil
}

// DeleteTenant removes a tenant from the configuration store.
func (c *TopLevelConfig) DeleteTenant(name string) error {
	_, err := c.etcdClient.Delete(context.Background(), c.tenant(name), nil)
	return err
}

// GetTenant retrieves a tenant from the configuration store.
func (c *TopLevelConfig) GetTenant(name string) (*TenantConfig, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.tenant(name), nil)
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
	resp, err := c.etcdClient.Get(context.Background(), c.prefixed(rootTenant), &client.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		return nil, err
	}

	if resp.Node == nil {
		return nil, fmt.Errorf("Tenant's root is missing")
	}

	tenants := []string{}

	for _, node := range resp.Node.Nodes {
		tenants = append(tenants, path.Base(node.Key))
	}

	return tenants, nil
}

// Validate ensures the structure of the tenant is sane.
func (cfg *TenantConfig) Validate() error {
	if cfg.FileSystems == nil {
		cfg.FileSystems = defaultFilesystems
	}

	return cfg.DefaultVolumeOptions.Validate()
}
