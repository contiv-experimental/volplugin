package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// VolumeConfig is the configuration of the tenant. It includes pool and
// snapshot information.
type VolumeConfig struct {
	VolumeName string        `json:"name"`
	Options    VolumeOptions `json:"options"`
}

// VolumeOptions comprises the optional paramters a volume can accept.
type VolumeOptions struct {
	Pool         string         `json:"pool" merge:"pool"`
	Size         uint64         `json:"size" merge:"size"`
	UseSnapshots bool           `json:"snapshots" merge:"snapshots"`
	Snapshot     SnapshotConfig `json:"snapshot"`
}

// SnapshotConfig is the configuration for snapshots.
type SnapshotConfig struct {
	Frequency string `json:"frequency" merge:"snapshots.frequency"`
	Keep      uint   `json:"keep" merge:"snapshots.keep"`
}

func (c *TopLevelConfig) volume(tenant, name string) string {
	return c.prefixed(rootVolume, tenant, name)
}

// CreateVolume sets the appropriate config metadata for a volume creation
// operation, and returns the VolumeConfig that was copied in.
func (c *TopLevelConfig) CreateVolume(rc RequestCreate) (*VolumeConfig, error) {
	if tc, err := c.GetVolume(rc.Tenant, rc.Volume); err == nil {
		return tc, ErrExist
	}

	resp, err := c.GetTenant(rc.Tenant)
	if err != nil {
		return nil, err
	}

	if err := mergeOpts(&resp.DefaultVolumeOptions, rc.Opts); err != nil {
		return nil, err
	}

	vc := VolumeConfig{
		Options:    resp.DefaultVolumeOptions,
		VolumeName: rc.Volume,
	}

	if vc.Options.Pool == "" {
		vc.Options.Pool = resp.DefaultPool
	}

	remarshal, err := json.Marshal(vc)
	if err != nil {
		return nil, err
	}

	if _, err := c.etcdClient.Set(c.volume(rc.Tenant, rc.Volume), string(remarshal), 0); err != nil {
		return nil, err
	}

	return &vc, nil
}

// GetVolume returns the VolumeConfig for a given volume.
func (c *TopLevelConfig) GetVolume(tenant, name string) (*VolumeConfig, error) {
	resp, err := c.etcdClient.Get(c.volume(tenant, name), true, false)
	if err != nil {
		return nil, err
	}

	ret := &VolumeConfig{}

	if err := json.Unmarshal([]byte(resp.Node.Value), ret); err != nil {
		return nil, err
	}

	return ret, nil
}

// RemoveVolume removes a volume from configuration.
func (c *TopLevelConfig) RemoveVolume(tenant, name string) error {
	_, err := c.etcdClient.Delete(c.prefixed(rootVolume, tenant, name), true)
	return err
}

// ListVolumes returns a map of volume name -> VolumeConfig.
func (c *TopLevelConfig) ListVolumes(tenant string) (map[string]*VolumeConfig, error) {
	tenantPath := c.prefixed(rootVolume, tenant)

	resp, err := c.etcdClient.Get(tenantPath, true, true)
	if err != nil {
		return nil, err
	}

	configs := map[string]*VolumeConfig{}

	for _, node := range resp.Node.Nodes {
		if node.Value == "" {
			continue
		}

		config := &VolumeConfig{}
		if err := json.Unmarshal([]byte(node.Value), config); err != nil {
			return nil, err
		}

		key := strings.TrimPrefix(node.Key, tenantPath)
		// trim leading slash
		configs[key[1:]] = config
	}

	return configs, nil
}

// ListAllVolumes returns an array with all the named tenants and volumes the
// volmaster knows about. Volumes have syntax: tenant/volumeName which will be
// reflected in the returned string.
func (c *TopLevelConfig) ListAllVolumes() ([]string, error) {
	resp, err := c.etcdClient.Get(c.prefixed(rootVolume), true, true)
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, node := range resp.Node.Nodes {
		key := strings.TrimPrefix(node.Key, c.prefixed(rootVolume))
		// trim leading slash
		ret = append(ret, key[1:])
	}

	return ret, nil
}

// Validate options for a volume. Should be called anytime options are
// considered.
func (opts VolumeOptions) Validate() error {
	if opts.Size == 0 {
		return fmt.Errorf("Config for tenant has a zero size")
	}

	if opts.UseSnapshots && (opts.Snapshot.Frequency == "" || opts.Snapshot.Keep == 0) {
		return fmt.Errorf("Snapshots are configured but cannot be used due to blank settings")
	}

	return nil
}

// Validate validates a volume configuration, returning error on any issue.
func (cfg *VolumeConfig) Validate() error {
	return cfg.Options.Validate()
}
