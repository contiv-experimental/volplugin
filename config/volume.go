package config

import (
	"encoding/json"
	"fmt"
)

func (c *TopLevelConfig) CreateVolume(name string, tenant string) (*TenantConfig, error) {
	if _, err := c.GetVolume(name); err == nil {
		return nil, fmt.Errorf("Volume %q is already in use", name)
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

func (c *TopLevelConfig) RemoveVolume(name string) error {
	_, err := c.etcdClient.Delete(c.prefixed("volumes", name), true)
	return err
}
