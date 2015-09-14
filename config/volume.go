package config

import (
	"encoding/json"
	"path"
)

// CreateVolume sets the appropriate config metadata for a volume creation
// operation, and returns the TenantConfig that was copied in.
func (c *TopLevelConfig) CreateVolume(name string, tenant string) (*TenantConfig, error) {
	if tc, err := c.GetVolume(name); err == nil {
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

	if _, err := c.etcdClient.Set(c.prefixed("volumes", name), resp, 0); err != nil {
		return nil, err
	}

	return ret, nil
}

// GetVolume returns the TenantConfig for a given volume.
func (c *TopLevelConfig) GetVolume(name string) (*TenantConfig, error) {
	resp, err := c.etcdClient.Get(c.prefixed("volumes", name), true, false)
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
func (c *TopLevelConfig) RemoveVolume(name string) error {
	_, err := c.etcdClient.Delete(c.prefixed("volumes", name), true)
	return err
}

// ListVolumes returns a map of volume name -> TenantConfig.
func (c *TopLevelConfig) ListVolumes() (map[string]*TenantConfig, error) {
	resp, err := c.etcdClient.Get(c.prefixed("volumes"), true, true)
	if err != nil {
		return nil, err
	}

	configs := map[string]*TenantConfig{}

	for _, node := range resp.Node.Nodes {
		config := &TenantConfig{}
		if err := json.Unmarshal([]byte(node.Value), config); err != nil {
			return nil, err
		}

		configs[path.Base(node.Key)] = config
	}

	return configs, nil
}
