package config

import (
	"encoding/json"
	"path"
	"strings"
)

// MountConfig is the exchange configuration for mounts. The payload is stored
// in etcd and used for comparison.
type MountConfig struct {
	Volume     string
	Pool       string
	MountPoint string
	Host       string
}

// ExistsMount checks if a mount exists
func (c *TopLevelConfig) ExistsMount(mt *MountConfig) bool {
	// skipping the error because we don't need it
	resp, err := c.etcdClient.Get(c.prefixed("mounts", mt.Pool, mt.Volume), true, false)
	return err == nil && resp.Node != nil
}

// PublishMount pushes the mount to etcd. Fails with ErrExist if the mount exists.
func (c *TopLevelConfig) PublishMount(mt *MountConfig) error {
	// FIXME this should use CompareAndSwap to avoid using a necessary mutex
	if c.ExistsMount(mt) {
		return ErrExist
	}

	content, err := json.Marshal(mt)
	if err != nil {
		return err
	}

	// FIXME the TTL here should be variable and there should be a way to refresh it.
	// This way if an instance goes down, its mount expires after a while.
	_, err = c.etcdClient.Set(c.prefixed("mounts", mt.Pool, mt.Volume), string(content), 0)
	return err
}

// RemoveMount will remove a mount from etcd. Does not fail if the mount does
// not exist.
func (c *TopLevelConfig) RemoveMount(mt *MountConfig) error {
	if !c.ExistsMount(mt) {
		// if we don't exist, do nothing!
		return nil
	}

	_, err := c.etcdClient.Delete(c.prefixed("mounts", mt.Pool, mt.Volume), true)
	return err
}

// GetMount retrieves the MountConfig for the given volume name.
func (c *TopLevelConfig) GetMount(volumeName string) (*MountConfig, error) {
	mt := &MountConfig{}

	resp, err := c.etcdClient.Get(c.prefixed("mounts", mt.Pool, mt.Volume), true, false)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(resp.Node.Value), mt); err != nil {
		return nil, err
	}

	return mt, nil
}

// ListMounts lists the mounts in use.
func (c *TopLevelConfig) ListMounts() ([]string, error) {
	resp, err := c.etcdClient.Get(c.prefixed("mounts"), true, true)
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, node := range resp.Node.Nodes {
		for _, inner := range node.Nodes {
			key := path.Join(strings.TrimPrefix(inner.Key, c.prefixed("mounts")))
			// trim leading slash
			key = key[1:]
			ret = append(ret, key)
		}
	}

	return ret, nil
}
