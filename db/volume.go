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
		vr.Policy.DriverOptions = map[string]string{}
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

// Hooks provides hooks into the volume CRUD lifecycle. Currently this is used
// to split runtime parameters out from the rest of the volume information.
func (v *Volume) Hooks() *Hooks {
	return &Hooks{
		PostGet: func(c Client, obj Entity) error {
			vol := obj.(*Volume)
			ro := vol.RuntimeOptions // pointer
			ro.policyName = vol.PolicyName
			ro.volumeName = vol.VolumeName
			return c.Get(ro)
		},
		PreSet: func(c Client, obj Entity) error {
			copy := obj.Copy()
			if err := c.Get(copy); err == nil {
				return errored.Errorf("%v", obj).Combine(errors.Exists)
			}

			vol := obj.(*Volume)
			ro := vol.RuntimeOptions // pointer
			ro.policyName = vol.PolicyName
			ro.volumeName = vol.VolumeName
			return c.Set(ro)
		},
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

	do, err := v.ToDriverOptions(time.Second)
	if err != nil {
		return err
	}

	// We use a few dummy variables to ensure that global configuration is
	// not needed in the storage drivers, that the validation does not fail
	// because of it.
	var mountPath string
	for driverType, backendName := range map[string]string{backend.Mount: v.Backends.Mount, backend.CRUD: v.Backends.CRUD, backend.Snapshot: v.Backends.Snapshot} {
		if backendName == "" {
			continue
		}

		if driverType == backend.Mount {
			mountPath = backend.MountPath
		} else {
			mountPath = ""
		}

		if err := backend.NewDriver(backendName, driverType, mountPath, &do); err != nil {
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
