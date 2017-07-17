package config

import (
	"encoding/json"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/merge"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"
	"github.com/contiv/volplugin/watch"
	"github.com/docker/go-units"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// Volume is the configuration of the policy. It includes pool and
// snapshot information.
type Volume struct {
	PolicyName     string               `json:"policy"`
	VolumeName     string               `json:"name"`
	Unlocked       bool                 `json:"unlocked,omitempty" merge:"unlocked"`
	DriverOptions  storage.DriverParams `json:"driver"`
	MountSource    string               `json:"mount" merge:"mount"`
	CreateOptions  CreateOptions        `json:"create"`
	RuntimeOptions RuntimeOptions       `json:"runtime"`
	Backends       *BackendDrivers      `json:"backends,omitempty"`
}

// CreateOptions are the set of options used by apiserver during the volume
// create operation.
type CreateOptions struct {
	Size       string `json:"size" merge:"size"`
	FileSystem string `json:"filesystem" merge:"filesystem"`
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
	WriteBPS uint64 `json:"write-bps" merge:"rate-limit.write.bps"`
	ReadBPS  uint64 `json:"read-bps" merge:"rate-limit.read.bps"`
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
	if err := ro.ValidateJSON(); err != nil {
		return errors.ErrJSONValidation.Combine(err)
	}

	content, err := json.Marshal(ro)
	if err != nil {
		return err
	}

	c.etcdClient.Set(context.Background(), c.prefixed(rootVolume, vo.PolicyName, vo.VolumeName), "", &client.SetOptions{Dir: true})
	if _, err := c.etcdClient.Set(context.Background(), c.volume(vo.PolicyName, vo.VolumeName, "runtime"), string(content), nil); err != nil {
		return errors.EtcdToErrored(err)
	}

	return nil
}

// CreateVolume sets the appropriate config metadata for a volume creation
// operation, and returns the Volume that was copied in.
func (c *Client) CreateVolume(rc *VolumeRequest) (*Volume, error) {
	resp, err := c.GetPolicy(rc.Policy)
	if err != nil {
		return nil, err
	}

	var mount string

	if rc.Options != nil {
		mount = rc.Options["mount"]
		delete(rc.Options, "mount")
	}

	if err := merge.Opts(resp, rc.Options); err != nil {
		return nil, err
	}

	if resp.DriverOptions == nil {
		resp.DriverOptions = storage.DriverParams{}
	}

	if err := resp.Validate(); err != nil {
		return nil, err
	}

	vc := &Volume{
		Backends:       resp.Backends,
		DriverOptions:  resp.DriverOptions,
		CreateOptions:  resp.CreateOptions,
		RuntimeOptions: resp.RuntimeOptions,
		Unlocked:       resp.Unlocked,
		PolicyName:     rc.Policy,
		VolumeName:     rc.Name,
		MountSource:    mount,
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
		return errors.Exists
	}

	return c.PublishVolumeRuntime(vc, vc.RuntimeOptions)
}

// ActualSize returns the size of the volume as an integer of megabytes.
func (co *CreateOptions) ActualSize() (uint64, error) {
	sizeStr := co.Size

	if strings.TrimSpace(sizeStr) == "" {
		sizeStr = "0"
	}

	size, err := units.FromHumanSize(sizeStr)
	// MB is the base unit for RBD
	return uint64(size) / units.MB, err
}

// GetVolume returns the Volume for a given volume.
func (c *Client) GetVolume(policy, name string) (*Volume, error) {
	// FIXME make this take a single string and not a split one
	resp, err := c.etcdClient.Get(context.Background(), c.volume(policy, name, "create"), nil)
	if err != nil {
		return nil, errors.EtcdToErrored(err)
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
		return runtime, errors.EtcdToErrored(err)
	}

	return runtime, json.Unmarshal([]byte(resp.Node.Value), &runtime)
}

// RemoveVolume removes a volume from configuration.
func (c *Client) RemoveVolume(policy, name string) error {
	logrus.Debugf("Removing volume %s/%s from database", policy, name)
	_, err := c.etcdClient.Delete(context.Background(), c.prefixed(rootVolume, policy, name), &client.DeleteOptions{Recursive: true})
	return errors.EtcdToErrored(err)
}

// ListVolumes returns a map of volume name -> Volume.
func (c *Client) ListVolumes(policy string) (map[string]*Volume, error) {
	policyPath := c.prefixed(rootVolume, policy)

	resp, err := c.etcdClient.Get(context.Background(), policyPath, &client.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		return nil, errors.EtcdToErrored(err)
	}

	configs := map[string]*Volume{}

	for _, node := range resp.Node.Nodes {
		if len(node.Nodes) > 0 {
			node = node.Nodes[0]
			key := strings.TrimPrefix(node.Key, policyPath)
			if !node.Dir && strings.HasSuffix(node.Key, "/create") {
				key = strings.TrimSuffix(key, "/create")

				config, ok := configs[key[1:]]
				if !ok {
					config = new(Volume)
				}

				if err := json.Unmarshal([]byte(node.Value), config); err != nil {
					return nil, err
				}
				// trim leading slash
				configs[key[1:]] = config
			}

			if !node.Dir && strings.HasSuffix(node.Key, "/runtime") {
				key = strings.TrimSuffix(key, "/create")

				config, ok := configs[key[1:]]
				if !ok {
					config = new(Volume)
				}

				if err := json.Unmarshal([]byte(node.Value), &config.RuntimeOptions); err != nil {
					return nil, err
				}
				// trim leading slash
				configs[key[1:]] = config
			}
		}
	}

	for _, config := range configs {
		if _, err := config.CreateOptions.ActualSize(); err != nil {
			return nil, err
		}
	}

	return configs, nil
}

// ListAllVolumes returns an array with all the named policies and volumes the
// apiserver knows about. Volumes have syntax: policy/volumeName which will be
// reflected in the returned string.
func (c *Client) ListAllVolumes() ([]string, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.prefixed(rootVolume), &client.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		if er, ok := errors.EtcdToErrored(err).(*errored.Error); ok && er.Contains(errors.NotExists) {
			return []string{}, nil
		}

		return nil, errors.EtcdToErrored(err)
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
		vw := &watch.Watch{Key: strings.TrimPrefix(resp.Node.Key, c.prefixed(rootVolume)+"/")}

		if !resp.Node.Dir && path.Base(resp.Node.Key) == "runtime" {
			logrus.Debugf("Handling watch event %q for volume %q", resp.Action, vw.Key)
			if resp.Action != "delete" {
				if resp.Node.Value != "" {
					volName := strings.TrimPrefix(path.Dir(vw.Key), c.prefixed(rootVolume)+"/")

					policy, vol := path.Split(volName)
					vw.Key = volName
					volume, err := c.GetVolume(policy, vol)
					if err != nil {
						logrus.Errorf("Could not retrieve volume %q after watch notification: %v", volName, err)
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

// TakeSnapshot immediately takes a snapshot by signaling the volsupervisor through etcd.
func (c *Client) TakeSnapshot(name string) error {
	_, err := c.etcdClient.Set(context.Background(), c.prefixed(rootSnapshots, name), "", nil)
	return errors.EtcdToErrored(err)
}

// RemoveTakeSnapshot removes a reference to a taken snapshot, intended to be used by volsupervisor
func (c *Client) RemoveTakeSnapshot(name string) error {
	_, err := c.etcdClient.Delete(context.Background(), c.prefixed(rootSnapshots, name), nil)
	return errors.EtcdToErrored(err)
}

// WatchSnapshotSignal watches for a signal to be provided to
// /volplugin/snapshots via writing an empty file to the policy/volume name.
func (c *Client) WatchSnapshotSignal(activity chan *watch.Watch) {
	w := watch.NewWatcher(activity, c.prefixed(rootSnapshots), func(resp *client.Response, w *watch.Watcher) {

		if !resp.Node.Dir && resp.Action != "delete" {
			vw := &watch.Watch{Key: strings.Replace(resp.Node.Key, c.prefixed(rootSnapshots)+"/", "", -1), Config: nil}
			w.Channel <- vw
		}
	})

	watch.Create(w)
}

// Validate validates a volume configuration, returning error on any issue.
func (cfg *Volume) Validate() error {
	if err := cfg.ValidateJSON(); err != nil {
		return errors.ErrJSONValidation.Combine(err)
	}

	return cfg.validateBackends()
}

func (cfg *Volume) validateBackends() error {
	// We use a few dummy variables to ensure that global configuration is
	// not needed in the storage drivers, that the validation does not fail
	// because of it.
	do, err := cfg.ToDriverOptions(time.Second)
	if err != nil {
		return err
	}

	if cfg.Backends.CRUD != "" {
		crud, err := backend.NewCRUDDriver(cfg.Backends.CRUD)
		if err != nil {
			return err
		}

		if err := crud.Validate(&do); err != nil {
			return err
		}
	}

	mnt, err := backend.NewMountDriver(cfg.Backends.Mount, backend.MountPath)
	if err != nil {
		return err
	}

	if err := mnt.Validate(&do); err != nil {
		return err
	}
	if cfg.Backends.Snapshot != "" {
		snapshot, err := backend.NewSnapshotDriver(cfg.Backends.Snapshot)
		if err != nil {
			return err
		}
		if err := snapshot.Validate(&do); err != nil {

			return err
		}
	}
	return nil
}

// ToDriverOptions converts a volume to a storage.DriverOptions.
func (cfg *Volume) ToDriverOptions(timeout time.Duration) (storage.DriverOptions, error) {
	actualSize, err := cfg.CreateOptions.ActualSize()
	if err != nil {
		return storage.DriverOptions{}, err
	}

	return storage.DriverOptions{
		Volume: storage.Volume{
			Name:   cfg.String(),
			Size:   actualSize,
			Params: cfg.DriverOptions,
		},
		FSOptions: storage.FSOptions{
			Type: cfg.CreateOptions.FileSystem,
		},
		Timeout: timeout,
		Source:  cfg.MountSource,
	}, nil
}

func (cfg *Volume) String() string {
	return path.Join(cfg.PolicyName, cfg.VolumeName)
}

// IsVolumeInUse checks if the given volume is mounted in any container
func (c *Client) IsVolumeInUse(cfg *Volume, global *Global) (bool, error) {
	if !cfg.Unlocked {
		uc := &UseMount{Volume: cfg.String()} // fields are deliberately cleared
		if err := c.PublishUse(uc); err != nil {
			if er, ok := err.(*errored.Error); ok {
				return er.Contains(errors.Exists), c.RemoveUse(uc, false)
			}

			logrus.Errorf("Error received checking for volume in-use status: %v", err)

			return false, c.RemoveUse(uc, false)
		}

		return false, c.RemoveUse(uc, false)
	}

	// XXX To simplify the behavior, always return true for unlocked volumes as there are no mount lock exists for them.
	return true, nil
}
