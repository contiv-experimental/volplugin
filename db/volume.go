package db

import (
	"path"
	"strings"
	"time"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/merge"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"
)

// NewVolume constructs a new volume given the policy and volume parameters.
func NewVolume(policy, volume string) *Volume {
	return &Volume{PolicyName: policy, VolumeName: volume}
}

// CreateVolume creates a volume from parameters, including the policy to copy.
func CreateVolume(vr *VolumeRequest) (*Volume, error) {
	if vr.Name == "" {
		return nil, errored.Errorf("Volume name was empty").Combine(errors.InvalidVolume)
	}

	if vr.Policy == nil {
		return nil, errored.Errorf("Policy for volume %q was nil", vr.Name).Combine(errors.InvalidVolume)
	}

	var mount string

	if vr.Options != nil {
		mount = vr.Options["mount"]
		delete(vr.Options, "mount")
	}

	if err := merge.Opts(vr.Policy, vr.Options); err != nil {
		return nil, err
	}

	if vr.Policy.DriverOptions == nil {
		vr.Policy.DriverOptions = storage.DriverParams{}
	}

	if err := vr.Policy.Validate(); err != nil {
		return nil, err
	}

	vc := &Volume{
		Backends:       vr.Policy.Backends,
		DriverOptions:  vr.Policy.DriverOptions,
		CreateOptions:  vr.Policy.CreateOptions,
		RuntimeOptions: vr.Policy.RuntimeOptions,
		Unlocked:       vr.Policy.Unlocked,
		PolicyName:     vr.Policy.Name,
		VolumeName:     vr.Name,
		MountSource:    mount,
	}

	if err := vc.Validate(); err != nil {
		return nil, err
	}

	if vc.CreateOptions.FileSystem == "" {
		vc.CreateOptions.FileSystem = DefaultFilesystem
	}

	return vc, nil
}

// SetKey implements the entity interface.
func (v *Volume) SetKey(key string) error {
	suffix := strings.Trim(strings.TrimPrefix(strings.Trim(key, "/"), rootVolume), "/")
	parts := strings.Split(suffix, "/")
	if len(parts) != 2 {
		return errors.InvalidDBPath.Combine(errored.Errorf("Args to SetKey for Volume were invalid: %v", key))
	}

	if parts[0] == "" || parts[1] == "" {
		return errors.InvalidDBPath.Combine(errored.Errorf("One part of key %v in Volume was empty: %v", key, parts))
	}

	v.PolicyName = parts[0]
	v.VolumeName = parts[1]

	return v.RuntimeOptions.SetKey(suffix)
}

// Prefix provides the prefix for the volumes root.
func (v *Volume) Prefix() string {
	return rootVolume
}

// Path provides the path to this volumes data store.
func (v *Volume) Path() (string, error) {
	if v.PolicyName == "" || v.VolumeName == "" {
		return "", errors.InvalidVolume.Combine(errored.New("Volume or policy name is missing"))
	}

	return strings.Join([]string{v.Prefix(), v.PolicyName, v.VolumeName}, "/"), nil
}

func (v *Volume) postGetHook(c Client, obj Entity) error {
	vol := obj.(*Volume)
	ro := vol.RuntimeOptions // pointer
	ro.policyName = vol.PolicyName
	ro.volumeName = vol.VolumeName
	return c.Get(ro)
}

func (v *Volume) preSetHook(c Client, obj Entity) error {
	copy := obj.Copy()
	if err := c.Get(copy); err == nil {
		return errored.Errorf("%v", obj).Combine(errors.Exists)
	}

	vol := obj.(*Volume)
	ro := vol.RuntimeOptions // pointer
	ro.policyName = vol.PolicyName
	ro.volumeName = vol.VolumeName
	return c.Set(ro)
}

// Hooks provides hooks into the volume CRUD lifecycle. Currently this is used
// to split runtime parameters out from the rest of the volume information.
func (v *Volume) Hooks() *Hooks {
	return &Hooks{
		PostGet: v.postGetHook,
		PreSet:  v.preSetHook,
	}
}

// Copy returns a deep copy of the volume.
func (v *Volume) Copy() Entity {
	v2 := *v
	if v.RuntimeOptions != nil {
		ro := *(v.RuntimeOptions)
		v2.RuntimeOptions = &ro
	}

	if v.Backends != nil {
		be := *(v.Backends)
		v2.Backends = &be
	}

	return &v2
}

// Validate validates the structure of the volume.
func (v *Volume) Validate() error {
	if err := validateJSON(VolumeSchema, v); err != nil {
		return errors.ErrJSONValidation.Combine(err)
	}

	return v.validateBackends() // calls ToDriverOptions.
}

func (v *Volume) validateBackends() error {
	// We use a few dummy variables to ensure that global configuration is
	// not needed in the storage drivers, that the validation does not fail
	// because of it.
	do, err := v.ToDriverOptions(time.Second)
	if err != nil {
		return err
	}

	if v.Backends.CRUD != "" {
		crud, err := backend.NewCRUDDriver(v.Backends.CRUD)
		if err != nil {
			return err
		}

		if err := crud.Validate(&do); err != nil {
			return err
		}
	}

	mnt, err := backend.NewMountDriver(v.Backends.Mount, backend.MountPath)
	if err != nil {
		return err
	}

	if err := mnt.Validate(&do); err != nil {
		return err
	}
	if v.Backends.Snapshot != "" {
		snapshot, err := backend.NewSnapshotDriver(v.Backends.Snapshot)
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
func (v *Volume) ToDriverOptions(timeout time.Duration) (storage.DriverOptions, error) {
	actualSize, err := v.CreateOptions.ActualSize()
	if err != nil {
		return storage.DriverOptions{}, err
	}

	return storage.DriverOptions{
		Volume: storage.Volume{
			Name:   v.String(),
			Size:   actualSize,
			Params: v.DriverOptions,
		},
		FSOptions: storage.FSOptions{
			Type: v.CreateOptions.FileSystem,
		},
		Timeout: timeout,
		Source:  v.MountSource,
	}, nil
}

func (v *Volume) String() string {
	return path.Join(v.PolicyName, v.VolumeName)
}
