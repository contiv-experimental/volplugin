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
	PolicyName     string            `json:"policy"`
	VolumeName     string            `json:"name"`
	DriverOptions  map[string]string `json:"driver"`
	CreateOptions  CreateOptions     `json:"create"`
	RuntimeOptions RuntimeOptions    `json:"runtime"`
	Backend        string            `json:"backend"`
}

// CreateOptions are the set of options used by volmaster during the volume
// create operation.
type CreateOptions struct {
	Size       string `json:"size" merge:"size"`
	FileSystem string `json:"filesystem" merge:"filesystem"`

	actualSize units.Base2Bytes
}

// RuntimeOptions are the set of options used by volplugin when mounting the
// volume, and by volsupervisor for calculating periodic work.
type RuntimeOptions struct {
	UseSnapshots bool            `json:"snapshots" merge:"snapshots"`
	Snapshot     SnapshotConfig  `json:"snapshot"`
	RateLimit    RateLimitConfig `json:"rate-limit,omitempty"`
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

func (c *Client) volume(policy, name, typ string) string {
	return c.prefixed(rootVolume, policy, name, typ)
}

// PublishVolumeRuntime publishes the runtime parameters for each volume.
func (c *Client) PublishVolumeRuntime(vo *Volume, ro RuntimeOptions) error {
	content, err := json.Marshal(ro)
	if err != nil {
		return err
	}

	c.etcdClient.Set(context.Background(), c.prefixed(rootVolume, vo.PolicyName, vo.VolumeName), "", &client.SetOptions{Dir: true})
	if _, err := c.etcdClient.Set(context.Background(), c.volume(vo.PolicyName, vo.VolumeName, "runtime"), string(content), nil); err != nil {
		return err
	}

	return nil
}

// CreateVolume sets the appropriate config metadata for a volume creation
// operation, and returns the Volume that was copied in.
func (c *Client) CreateVolume(rc RequestCreate) (*Volume, error) {
	resp, err := c.GetPolicy(rc.Policy)
	if err != nil {
		return nil, err
	}

	if err := mergeOpts(resp, rc.Opts); err != nil {
		return nil, err
	}

	if err := resp.Validate(); err != nil {
		return nil, err
	}

	vc := &Volume{
		Backend:        resp.Backend,
		DriverOptions:  resp.DriverOptions,
		CreateOptions:  resp.CreateOptions,
		RuntimeOptions: resp.RuntimeOptions,
		PolicyName:     rc.Policy,
		VolumeName:     rc.Volume,
	}

	if err := vc.Validate(); err != nil {
		return nil, err
	}

	if vc.CreateOptions.FileSystem == "" {
		vc.CreateOptions.FileSystem = defaultFilesystem
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

	c.etcdClient.Set(context.Background(), c.prefixed(rootVolume, vc.PolicyName, vc.VolumeName), "", &client.SetOptions{Dir: true})

	if _, err := c.etcdClient.Set(context.Background(), c.volume(vc.PolicyName, vc.VolumeName, "create"), string(remarshal), &client.SetOptions{PrevExist: client.PrevNoExist}); err != nil {
		return ErrExist
	}

	return c.PublishVolumeRuntime(vc, vc.RuntimeOptions)
}

// ActualSize returns the size of the volume as an integer of megabytes.
func (co *CreateOptions) ActualSize() (uint64, error) {
	if err := co.computeSize(); err != nil {
		return 0, err
	}
	return uint64(co.actualSize), nil
}

func (co *CreateOptions) computeSize() error {
	var err error

	co.actualSize, err = units.ParseBase2Bytes(co.Size)
	if err != nil {
		return err
	}

	if co.actualSize != 0 {
		co.actualSize = co.actualSize / units.Mebibyte
	}

	return nil
}

// GetVolume returns the Volume for a given volume.
func (c *Client) GetVolume(policy, name string) (*Volume, error) {
	// FIXME make this take a single string and not a split one
	resp, err := c.etcdClient.Get(context.Background(), c.volume(policy, name, "create"), nil)
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

	runtime, err := c.GetVolumeRuntime(policy, name)
	if err != nil {
		return nil, err
	}

	ret.RuntimeOptions = runtime

	return ret, nil
}

// GetVolumeRuntime retrieves only the runtime parameters for the volume.
func (c *Client) GetVolumeRuntime(policy, name string) (RuntimeOptions, error) {
	runtime := RuntimeOptions{}

	resp, err := c.etcdClient.Get(context.Background(), c.volume(policy, name, "runtime"), nil)
	if err != nil {
		return runtime, err
	}

	return runtime, json.Unmarshal([]byte(resp.Node.Value), &runtime)
}

// RemoveVolume removes a volume from configuration.
func (c *Client) RemoveVolume(policy, name string) error {
	// FIXME might be a consistency issue here; pass around volume structs instead.
	_, err := c.etcdClient.Delete(context.Background(), c.prefixed(rootVolume, policy, name), &client.DeleteOptions{Recursive: true})
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
		if len(node.Nodes) > 0 {
			node = node.Nodes[0]
			if !node.Dir && strings.HasSuffix(node.Key, "/create") {
				config := &Volume{}
				if err := json.Unmarshal([]byte(node.Value), config); err != nil {
					return nil, err
				}

				key := strings.TrimPrefix(node.Key, policyPath)
				key = strings.TrimSuffix(key, "/create")
				// trim leading slash
				configs[key[1:]] = config
			}
		}
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

// WatchVolumeRuntimes watches the runtime portions of the volume and yields
// back any information received through the activity channel.
func (c *Client) WatchVolumeRuntimes(activity chan *watch.Watch) {
	w := watch.NewWatcher(activity, c.prefixed(rootVolume), func(resp *client.Response, w *watch.Watcher) {
		vw := &watch.Watch{Key: strings.Replace(resp.Node.Key, c.prefixed(rootVolume)+"/", "", -1), Config: nil}

		if !resp.Node.Dir && path.Base(resp.Node.Key) == "runtime" {
			log.Debugf("Handling watch event %q for volume %q", resp.Action, vw.Key)
			if resp.Action != "delete" {
				volume := &RuntimeOptions{}

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
					policy, vol := path.Split(path.Dir(resp.Node.Key))
					vw.Key = path.Join(path.Base(policy), vol)
					vw.Config = volume
				}
			}

			w.Channel <- vw
		}
	})

	watch.Create(w)
}

// WatchVolumeCreates watches the volumes tree and returns data back to the activity channel.
func (c *Client) WatchVolumeCreates(activity chan *watch.Watch) {
	w := watch.NewWatcher(activity, c.prefixed(rootVolume), func(resp *client.Response, w *watch.Watcher) {
		vw := &watch.Watch{Key: strings.Replace(resp.Node.Key, c.prefixed(rootVolume)+"/", "", -1), Config: nil}

		if !resp.Node.Dir && path.Base(resp.Node.Key) == "create" {
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
					policy, vol := path.Split(path.Dir(resp.Node.Key))
					vw.Key = path.Join(path.Base(policy), vol)
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
func (co *CreateOptions) Validate() error {
	if co.actualSize == 0 {
		actualSize, err := co.ActualSize()
		if err != nil {
			return err
		}

		if actualSize == 0 {
			return errored.Errorf("Config for policy has a zero size")
		}
	}

	return nil
}

// Validate options for a volume. Should be called anytime options are
// considered.
func (ro *RuntimeOptions) Validate() error {
	if ro.UseSnapshots && (ro.Snapshot.Frequency == "" || ro.Snapshot.Keep == 0) {
		return errored.Errorf("Snapshots are configured but cannot be used due to blank settings")
	}

	return nil
}

// Validate validates a volume configuration, returning error on any issue.
func (cfg *Volume) Validate() error {
	if cfg.Backend == "" {
		return errored.Errorf("No storage backend selected for volume %v", cfg)
	}

	if cfg.VolumeName == "" {
		return errored.Errorf("Volume Name was omitted for volume %v", cfg)
	}

	if cfg.PolicyName == "" {
		return errored.Errorf("Policy name was omitted for volume %v", cfg)
	}

	if err := cfg.CreateOptions.Validate(); err != nil {
		return err
	}

	return cfg.RuntimeOptions.Validate()
}

func (cfg *Volume) String() string {
	return path.Join(cfg.PolicyName, cfg.VolumeName)
}
