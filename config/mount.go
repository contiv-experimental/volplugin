package config

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/coreos/etcd/client"

	"golang.org/x/net/context"
)

// MountConfig is the exchange configuration for mounts. The payload is stored
// in etcd and used for comparison.
type MountConfig struct {
	Volume     string
	Pool       string
	MountPoint string
	Host       string
}

func (c *TopLevelConfig) mount(pool, name string) string {
	return c.prefixed(rootMount, pool, name)
}

// PublishMount pushes the mount to etcd. Fails with ErrExist if the mount exists.
func (c *TopLevelConfig) PublishMount(mt *MountConfig) error {
	content, err := json.Marshal(mt)
	if err != nil {
		return err
	}

	// FIXME the TTL here should be variable and there should be a way to refresh it.
	// This way if an instance goes down, its mount expires after a while.
	_, err = c.etcdClient.Set(context.Background(), c.mount(mt.Pool, mt.Volume), string(content), &client.SetOptions{PrevExist: client.PrevNoExist})
	return err
}

// RemoveMount will remove a mount from etcd. Does not fail if the mount does
// not exist.
func (c *TopLevelConfig) RemoveMount(mt *MountConfig, force bool) error {
	content, err := json.Marshal(mt)
	if err != nil {
		return err
	}

	opts := &client.DeleteOptions{PrevValue: string(content)}
	if force {
		opts = nil
	}

	_, err = c.etcdClient.Delete(context.Background(), c.mount(mt.Pool, mt.Volume), opts)
	return err
}

// GetMount retrieves the MountConfig for the given volume name.
func (c *TopLevelConfig) GetMount(pool, name string) (*MountConfig, error) {
	mt := &MountConfig{}

	resp, err := c.etcdClient.Get(context.Background(), c.mount(pool, name), nil)
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
	resp, err := c.etcdClient.Get(context.Background(), c.prefixed(rootMount), &client.GetOptions{Sort: true, Recursive: true})
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, node := range resp.Node.Nodes {
		for _, inner := range node.Nodes {
			key := path.Join(strings.TrimPrefix(inner.Key, c.prefixed(rootMount)))
			// trim leading slash
			key = key[1:]
			ret = append(ret, key)
		}
	}

	return ret, nil
}
