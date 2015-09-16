package config

import (
	"encoding/json"
	"strings"
)

// CreateVolume sets the appropriate config metadata for a volume creation
// operation, and returns the TenantConfig that was copied in.
func (c *TopLevelConfig) CreateVolume(name, tenant, pool string) (*TenantConfig, error) {
	if tc, err := c.GetVolume(name, pool); err == nil {
		return tc, ErrExist
	}

	resp, err := c.GetTenant(tenant)
	if err != nil {
		return nil, err
	}

	ret := &TenantConfig{}

	if err := json.Unmarshal([]byte(resp), ret); err != nil {
		return nil, err
	}

	if _, err := c.etcdClient.Set(c.prefixed("volumes", pool, name), resp, 0); err != nil {
		return nil, err
	}

	return ret, nil
}

// GetVolume returns the TenantConfig for a given volume.
func (c *TopLevelConfig) GetVolume(name, pool string) (*TenantConfig, error) {
	resp, err := c.etcdClient.Get(c.prefixed("volumes", pool, name), true, false)
	if err != nil {
		return nil, err
	}

	ret := &TenantConfig{}

	if err := json.Unmarshal([]byte(resp.Node.Value), ret); err != nil {
		return nil, err
	}

	return ret, nil
}

// RemoveVolume removes a volume from configuration.
func (c *TopLevelConfig) RemoveVolume(pool, name string) error {
	_, err := c.etcdClient.Delete(c.prefixed("volumes", pool, name), true)
	return err
}

// ListVolumes returns a map of volume name -> TenantConfig.
func (c *TopLevelConfig) ListVolumes(pool string) (map[string]*TenantConfig, error) {
	resp, err := c.etcdClient.Get(c.prefixed("volumes", pool), true, true)
	if err != nil {
		return nil, err
	}

	configs := map[string]*TenantConfig{}

	for _, node := range resp.Node.Nodes {
		if node.Value == "" {
			continue
		}

		config := &TenantConfig{}
		if err := json.Unmarshal([]byte(node.Value), config); err != nil {
			return nil, err
		}

		key := strings.TrimPrefix(node.Key, c.prefixed("volumes", pool))
		configs[key] = config
	}

	return configs, nil
}

// ListPools returns an array with all the named pools the volmaster knows
// about.
func (c *TopLevelConfig) ListPools() ([]string, error) {
	resp, err := c.etcdClient.Get(c.prefixed("volumes"), true, true)
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, node := range resp.Node.Nodes {
		if node.Value == "" {
			continue
		}

		key := strings.TrimPrefix(node.Key, c.prefixed("volumes"))
		ret = append(ret, key)
	}

	return ret, nil
}
