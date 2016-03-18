package config

import (
	"encoding/json"
	"path"
	"strings"
	"time"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/watch"

	log "github.com/Sirupsen/logrus"
	"github.com/alecthomas/units"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// Volume is the configuration of the policy. It includes pool and
// snapshot information.
type Volume struct {
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

func (c *Client) volume(policy, name string) string {
	return c.prefixed(rootVolume, policy, name)
}

// CreateVolume sets the appropriate config metadata for a volume creation
// operation, and returns the Volume that was copied in.
func (c *Client) CreateVolume(rc RequestCreate) (*Volume, error) {
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

	vc := &Volume{
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
func (c *Client) PublishVolume(vc *Volume) error {
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

// GetVolume returns the Volume for a given volume.
func (c *Client) GetVolume(policy, name string) (*Volume, error) {
	// FIXME make this take a single string and not a split one
	resp, err := c.etcdClient.Get(context.Background(), c.volume(policy, name), nil)
	if err != nil {
		return nil, err
	}

	ret := &Volume{}

	if err := json.Unmarshal([]byte(resp.Node.Value), ret); err != nil {
		return nil, err
	}

	if err := ret.Validate(); err != nil {
		return nil, err
	}

	return ret, nil
}

// RemoveVolume removes a volume from configuration.
func (c *Client) RemoveVolume(policy, name string) error {
	// FIXME might be a consistency issue here; pass around volume structs instead.
	_, err := c.etcdClient.Delete(context.Background(), c.prefixed(rootVolume, policy, name), &client.DeleteOptions{})
	return err
}

// ListVolumes returns a map of volume name -> Volume.
func (c *Client) ListVolumes(policy string) (map[string]*Volume, error) {
	policyPath := c.prefixed(rootVolume, policy)

	resp, err := c.etcdClient.Get(context.Background(), policyPath, &client.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		return nil, err
	}

	configs := map[string]*Volume{}

	for _, node := range resp.Node.Nodes {
		if node.Value == "" {
			continue
		}

		config := &Volume{}
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
func (c *Client) ListAllVolumes() ([]string, error) {
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

// WatchVolumes watches the volumes tree and returns data back to the activity channel.
func (c *Client) WatchVolumes(activity chan *watch.Watch) {
	w := watch.NewWatcher(activity, c.prefixed(rootVolume), func(resp *client.Response, w *watch.Watcher) {
		vw := &watch.Watch{Key: strings.Replace(resp.Node.Key, c.prefixed(rootVolume)+"/", "", -1), Config: nil}

		if !resp.Node.Dir {
			log.Debugf("Handling watch event %q for volume %q", resp.Action, vw.Key)
			if resp.Action != "delete" {
				volume := &Volume{}

				if resp.Node.Value != "" {
					if err := json.Unmarshal([]byte(resp.Node.Value), volume); err != nil {
						log.Errorf("Error decoding volume %q, not updating", resp.Node.Key)
						time.Sleep(1 * time.Second)
						return
					}

					if err := volume.Validate(); err != nil {
						log.Errorf("Error validating volume %q, not updating", resp.Node.Key)
						time.Sleep(1 * time.Second)
						return
					}
					vw.Config = volume
				}
			}

			w.Channel <- vw
		}
	})

	watch.Create(w)
}

// Validate options for a volume. Should be called anytime options are
// considered.
func (vo *VolumeOptions) Validate() error {
	if vo.Pool == "" {
		return errored.Errorf("No Pool specified")
	}

	if vo.actualSize == 0 {
		actualSize, err := vo.ActualSize()
		if err != nil {
			return err
		}

		if actualSize == 0 {
			return errored.Errorf("Config for policy has a zero size")
		}
	}

	if vo.UseSnapshots && (vo.Snapshot.Frequency == "" || vo.Snapshot.Keep == 0) {
		return errored.Errorf("Snapshots are configured but cannot be used due to blank settings")
	}

	return nil
}

// Validate validates a volume configuration, returning error on any issue.
func (cfg *Volume) Validate() error {
	if cfg.VolumeName == "" {
		return errored.Errorf("Volume Name was omitted")
	}

	if cfg.PolicyName == "" {
		return errored.Errorf("Policy name was omitted")
	}

	if cfg.Options == nil {
		return errored.Errorf("Options were omitted from volume creation")
	}

	return cfg.Options.Validate()
}

func (cfg *Volume) String() string {
	return path.Join(cfg.PolicyName, cfg.VolumeName)
}
