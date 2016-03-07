package config

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/alecthomas/units"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// VolumeConfig is the configuration of the policy. It includes pool and
// snapshot information.
type VolumeConfig struct {
	PolicyName string         `json:"policy"`
	VolumeName string         `json:"name"`
	Options    *VolumeOptions `json:"options"`
}

// VolumeOptions comprises the optional paramters a volume can accept.
type VolumeOptions struct {
	Pool         string          `json:"pool" merge:"pool"`
	Size         string          `json:"size" merge:"size"`
	UseSnapshots bool            `json:"snapshots" merge:"snapshots"`
	Snapshot     SnapshotConfig  `json:"snapshot"`
	FileSystem   string          `json:"filesystem" merge:"filesystem"`
	Ephemeral    bool            `json:"ephemeral,omitempty" merge:"ephemeral"`
	RateLimit    RateLimitConfig `json:"rate-limit,omitempty"`
	Backend      string          `json:"backend"`

	actualSize units.Base2Bytes
}

// RateLimitConfig is the configuration for limiting the rate of disk access.
type RateLimitConfig struct {
	WriteIOPS uint   `json:"write-iops" merge:"rate-limit.write.iops"`
	ReadIOPS  uint   `json:"read-iops" merge:"rate-limit.read.iops"`
	WriteBPS  uint64 `json:"write-bps" merge:"rate-limit.write.bps"`
	ReadBPS   uint64 `json:"read-bps" merge:"rate-limit.read.bps"`
}

// SnapshotConfig is the configuration for snapshots.
type SnapshotConfig struct {
	Frequency string `json:"frequency" merge:"snapshots.frequency"`
	Keep      uint   `json:"keep" merge:"snapshots.keep"`
}

func (c *TopLevelConfig) volume(policy, name string) string {
	return c.prefixed(rootVolume, policy, name)
}

// CreateVolume sets the appropriate config metadata for a volume creation
// operation, and returns the VolumeConfig that was copied in.
func (c *TopLevelConfig) CreateVolume(rc RequestCreate) (*VolumeConfig, error) {
	resp, err := c.GetPolicy(rc.Policy)
	if err != nil {
		return nil, err
	}

	if err := mergeOpts(&resp.DefaultVolumeOptions, rc.Opts); err != nil {
		return nil, err
	}

	if err := resp.Validate(); err != nil {
		return nil, err
	}

	vc := &VolumeConfig{
		Options:    &resp.DefaultVolumeOptions,
		PolicyName: rc.Policy,
		VolumeName: rc.Volume,
	}

	if err := vc.Validate(); err != nil {
		return nil, err
	}

	if vc.Options.FileSystem == "" {
		vc.Options.FileSystem = defaultFilesystem
	}

	return vc, nil
}

// PublishVolume writes a volume to etcd.
func (c *TopLevelConfig) PublishVolume(vc *VolumeConfig) error {
	if err := vc.Validate(); err != nil {
		return err
	}

	remarshal, err := json.Marshal(vc)
	if err != nil {
		return err
	}

	c.etcdClient.Set(context.Background(), c.prefixed(rootVolume, vc.PolicyName), "", &client.SetOptions{Dir: true})

	if _, err := c.etcdClient.Set(context.Background(), c.volume(vc.PolicyName, vc.VolumeName), string(remarshal), &client.SetOptions{PrevExist: client.PrevNoExist}); err != nil {
		return ErrExist
	}

	return nil
}

// ActualSize returns the size of the volume as an integer of megabytes.
func (vo *VolumeOptions) ActualSize() (uint64, error) {
	if err := vo.computeSize(); err != nil {
		return 0, err
	}
	return uint64(vo.actualSize), nil
}

func (vo *VolumeOptions) computeSize() error {
	var err error

	vo.actualSize, err = units.ParseBase2Bytes(vo.Size)
	if err != nil {
		return err
	}

	if vo.actualSize != 0 {
		vo.actualSize = vo.actualSize / units.Mebibyte
	}

	return nil
}

// GetVolume returns the VolumeConfig for a given volume.
func (c *TopLevelConfig) GetVolume(policy, name string) (*VolumeConfig, error) {
	// FIXME make this take a single string and not a split one
	resp, err := c.etcdClient.Get(context.Background(), c.volume(policy, name), nil)
	if err != nil {
		return nil, err
	}

	ret := &VolumeConfig{}

	if err := json.Unmarshal([]byte(resp.Node.Value), ret); err != nil {
		return nil, err
	}

	if err := ret.Validate(); err != nil {
		return nil, err
	}

	return ret, nil
}

// RemoveVolume removes a volume from configuration.
func (c *TopLevelConfig) RemoveVolume(policy, name string) error {
	// FIXME might be a consistency issue here; pass around volume structs instead.
	_, err := c.etcdClient.Delete(context.Background(), c.prefixed(rootVolume, policy, name), &client.DeleteOptions{})
	return err
}

// ListVolumes returns a map of volume name -> VolumeConfig.
func (c *TopLevelConfig) ListVolumes(policy string) (map[string]*VolumeConfig, error) {
	policyPath := c.prefixed(rootVolume, policy)

	resp, err := c.etcdClient.Get(context.Background(), policyPath, &client.GetOptions{Recursive: true, Sort: true})
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

		key := strings.TrimPrefix(node.Key, policyPath)
		// trim leading slash
		configs[key[1:]] = config
	}

	return configs, nil
}

// ListAllVolumes returns an array with all the named policies and volumes the
// volmaster knows about. Volumes have syntax: policy/volumeName which will be
// reflected in the returned string.
func (c *TopLevelConfig) ListAllVolumes() ([]string, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.prefixed(rootVolume), &client.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, node := range resp.Node.Nodes {
		for _, innerNode := range node.Nodes {
			ret = append(ret, path.Join(path.Base(node.Key), path.Base(innerNode.Key)))
		}
	}

	return ret, nil
}

// Validate options for a volume. Should be called anytime options are
// considered.
func (vo *VolumeOptions) Validate() error {
	if vo.Pool == "" {
		return fmt.Errorf("No Pool specified")
	}

	if vo.actualSize == 0 {
		actualSize, err := vo.ActualSize()
		if err != nil {
			return err
		}

		if actualSize == 0 {
			return fmt.Errorf("Config for policy has a zero size")
		}
	}

	if vo.UseSnapshots && (vo.Snapshot.Frequency == "" || vo.Snapshot.Keep == 0) {
		return fmt.Errorf("Snapshots are configured but cannot be used due to blank settings")
	}

	return nil
}

// Validate validates a volume configuration, returning error on any issue.
func (cfg *VolumeConfig) Validate() error {
	if cfg.VolumeName == "" {
		return fmt.Errorf("Volume Name was omitted")
	}

	if cfg.PolicyName == "" {
		return fmt.Errorf("Policy name was omitted")
	}

	if cfg.Options == nil {
		return fmt.Errorf("Options were omitted from volume creation")
	}

	return cfg.Options.Validate()
}

func (cfg *VolumeConfig) String() string {
	return path.Join(cfg.PolicyName, cfg.VolumeName)
}
