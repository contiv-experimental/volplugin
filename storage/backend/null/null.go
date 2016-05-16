// +build nope

package null

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/storage"
)

// Driver for null operations. Does nothing.
type Driver struct {
	mountpath string
}

// BackendName is the name of the driver
var BackendName = "null"

// NewCRUDDriver is a null form of the CRUD driver, that does nothing.
func NewCRUDDriver() (storage.CRUDDriver, error) {
	return &Driver{}, nil
}

// NewSnapshotDriver creates a new null snapshot driver.
func NewSnapshotDriver() (storage.SnapshotDriver, error) {
	return &Driver{}, nil
}

// NewMountDriver constructs a storage.MountDriver
func NewMountDriver(mountPath string) (storage.MountDriver, error) {
	return &Driver{mountpath: mountPath}, nil
}

// Name is the name of the driver
func (d *Driver) Name() string {
	return BackendName
}

// Create creates nothing.
func (d *Driver) Create(storage.DriverOptions) error {
	return nil
}

// Format formats nothing.
func (d *Driver) Format(storage.DriverOptions) error {
	return nil
}

// Destroy destroys nothing.
func (d *Driver) Destroy(storage.DriverOptions) error {
	return nil
}

// Exists does nothing.
func (d *Driver) Exists(storage.DriverOptions) (bool, error) {
	return false, nil
}

// List does nothing.
func (d *Driver) List(storage.ListOptions) ([]storage.Volume, error) {
	return []storage.Volume{}, nil
}

// CreateSnapshot does nothing.
func (d *Driver) CreateSnapshot(string, storage.DriverOptions) error {
	return nil
}

// RemoveSnapshot removes nothing.
func (d *Driver) RemoveSnapshot(string, storage.DriverOptions) error {
	return nil
}

// ListSnapshots lists nothing.
func (d *Driver) ListSnapshots(storage.DriverOptions) ([]string, error) {
	return []string{}, nil
}

// CopySnapshot copies nothing.
func (d *Driver) CopySnapshot(storage.DriverOptions, string, string) error {
	return nil
}

// Validate validates storaage.DriverOptions
func (d *Driver) Validate(storage.DriverOptions) error {
	return nil
}

// MountPath describes the path at which the volume should be mounted.
func (d *Driver) MountPath(do storage.DriverOptions) (string, error) {
	return filepath.Join(d.mountpath, do.Volume.Name), nil
}

// Mounted shows any volumes that belong to volplugin on the host, in
// their native representation. They yield a *Mount.
func (d *Driver) Mounted(time.Duration) ([]*storage.Mount, error) {
	mounts := []*storage.Mount{}
	fis, err := ioutil.ReadDir(d.mountpath)
	if os.IsNotExist(err) {
		return mounts, os.MkdirAll(d.mountpath, 0700)
	} else if err != nil {
		return nil, errored.Errorf("Reading policy tree for mounts").Combine(err)
	}

	for _, fi := range fis {
		volumes, err := ioutil.ReadDir(filepath.Join(d.mountpath, fi.Name()))
		if err != nil {
			return nil, errored.Errorf("Reading mounted volumes for policy %q", fi.Name()).Combine(err)
		}

		for _, vol := range volumes {
			rel := filepath.Join(d.mountpath, fi.Name(), vol.Name())
			if err != nil {
				return nil, errored.Errorf("Calculating mount information for %q/%q", fi.Name(), vol.Name()).Combine(err)
			}

			mounts = append(mounts, &storage.Mount{
				Path: rel,
				Volume: storage.Volume{
					Name:   rel,
					Source: "null",
				},
			})
		}
	}

	return mounts, nil
}

// Mount a Volume
func (d *Driver) Mount(do storage.DriverOptions) (*storage.Mount, error) {
	mp, err := d.MountPath(do)
	if err != nil {
		return nil, errored.Errorf("Calculating mount path for %q", do.Volume.Name).Combine(err)
	}

	if err := os.MkdirAll(mp, 0700); err != nil {
		return nil, errored.Errorf("Making mount path for %q", do.Volume.Name).Combine(err)
	}

	return &storage.Mount{
		Path: mp,
		Volume: storage.Volume{
			Name:   do.Volume.Name,
			Source: "null",
		},
	}, nil
}

// Unmount a volume
func (d *Driver) Unmount(do storage.DriverOptions) error {
	mp, err := d.MountPath(do)
	if err != nil {
		return errored.Errorf("Calculating mount path for %q", do.Volume.Name).Combine(err)
	}

	if err := os.RemoveAll(mp); err != nil {
		return errored.Errorf("Removing mount path for %q", do.Volume.Name).Combine(err)
	}

	return nil
}
