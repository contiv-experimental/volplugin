package nfs

import (
	"time"

	"github.com/contiv/volplugin/storage"
)

// Driver is a basic struct for controlling the NFS driver.
type Driver struct {
	mountpath string
}

// BackendName is the name of the driver.
const BackendName = "nfs"

// NewMountDriver constructs a new NFS driver.
func NewMountDriver(mountPath string) (storage.MountDriver, error) {
	return &Driver{mountpath: mountPath}, nil
}

// Name returns the string associated with the storage backed of the driver
func (d *Driver) Name() string { return BackendName }

// Create a volume.
func (d *Driver) Create(do storage.DriverOptions) error { return nil }

// Format a volume.
func (d *Driver) Format(do storage.DriverOptions) error { return nil }

// Destroy a volume.
func (d *Driver) Destroy(do storage.DriverOptions) error { return nil }

// List Volumes. May be scoped by storage parameters or other data.
func (d *Driver) List(lo storage.ListOptions) ([]storage.Volume, error) {
	return []storage.Volume{}, nil
}

// Mount a Volume
func (d *Driver) Mount(do storage.DriverOptions) (*storage.Mount, error) { return nil, nil }

// Unmount a volume
func (d *Driver) Unmount(do storage.DriverOptions) error { return nil }

// Exists returns true if a volume exists. Otherwise, it returns false.
func (d *Driver) Exists(do storage.DriverOptions) (bool, error) { return false, nil }

// CreateSnapshot creates a named snapshot for the volume. Any error will be returned.
func (d *Driver) CreateSnapshot(name string, do storage.DriverOptions) error { return nil }

// RemoveSnapshot removes a named snapshot for the volume. Any error will be returned.
func (d *Driver) RemoveSnapshot(name string, do storage.DriverOptions) error { return nil }

// ListSnapshots returns an array of snapshot names provided a maximum number
// of snapshots to be returned. Any error will be returned.
func (d *Driver) ListSnapshots(do storage.DriverOptions) ([]string, error) { return []string{}, nil }

// CopySnapshot copies a snapshot into a new volume. Takes a DriverOptions,
// snap and volume name (string). Returns error on failure.
func (d *Driver) CopySnapshot(do storage.DriverOptions, snapName string, volName string) error {
	return nil
}

// Mounted shows any volumes that belong to volplugin on the host, in
// their native representation. They yield a *Mount.
func (d *Driver) Mounted(timeout time.Duration) ([]*storage.Mount, error) {
	return []*storage.Mount{}, nil
}

// InternalName translates a volplugin `tenant/volume` name to an internal
// name suitable for the driver. Yields an error if impossible.
func (d *Driver) InternalName(volName string) (string, error) { return "", nil }

// InternalNameToVolpluginName translates an internal name to a volplugin
// `tenant/volume` syntax name.
func (d *Driver) InternalNameToVolpluginName(intName string) string { return "" }

// MountPath describes the path at which the volume should be mounted.
func (d *Driver) MountPath(do storage.DriverOptions) (string, error) { return d.mountpath, nil }
