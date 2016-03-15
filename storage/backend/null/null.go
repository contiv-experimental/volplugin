package null

import (
	"path/filepath"
	"time"

	"github.com/contiv/volplugin/storage"
)

// BackendName is string for no-op storage backend
const BackendName = "null"

// Driver implements a no-op storage driver for volplugin.
//
// This is intended for regressing volplugin
type Driver struct {
	mountpath string
}

// NewDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewDriver(mountpath string) storage.Driver {
	return &Driver{mountpath: mountpath}
}

// Name returns the null backend string
func (d *Driver) Name() string {
	return BackendName
}

// Create is a noop.
func (d *Driver) Create(storage.DriverOptions) error {
	return nil
}

// Format is a noop.
func (d *Driver) Format(do storage.DriverOptions) error {
	return nil
}

// Destroy is a noop.
func (d *Driver) Destroy(do storage.DriverOptions) error {
	return nil
}

// List Volumes returns empty list.
func (d *Driver) List(lo storage.ListOptions) ([]storage.Volume, error) {
	return []storage.Volume{}, nil
}

// Mount is a noop
func (d *Driver) Mount(do storage.DriverOptions) (*storage.Mount, error) {
	return nil, nil
}

// Unmount is a noop.
func (d *Driver) Unmount(do storage.DriverOptions) error {
	return nil
}

// Exists returns false always.
func (d *Driver) Exists(do storage.DriverOptions) (bool, error) {
	return false, nil
}

// CreateSnapshot is a noop.
func (d *Driver) CreateSnapshot(s string, do storage.DriverOptions) error {
	return nil
}

// RemoveSnapshot is a noop.
func (d *Driver) RemoveSnapshot(s string, do storage.DriverOptions) error {
	return nil
}

// CopySnapshot is a noop.
func (d *Driver) CopySnapshot(do storage.DriverOptions, s, s2 string) error {
	return nil
}

// ListSnapshots returns an empty list.
func (d *Driver) ListSnapshots(do storage.DriverOptions) ([]string, error) {
	return []string{}, nil
}

// Mounted retuns an empty list.
func (d *Driver) Mounted(t time.Duration) ([]*storage.Mount, error) {
	return []*storage.Mount{}, nil
}

// InternalName returns the passed string as is.
func (d *Driver) InternalName(s string) (string, error) {
	return s, nil
}

// InternalNameToVolpluginName returns the passed string as is.
func (d *Driver) InternalNameToVolpluginName(s string) string {
	return s
}

// MountPath describes the path at which the volume should be mounted.
func (d *Driver) MountPath(do storage.DriverOptions) string {
	return filepath.Join(d.mountpath, do.Volume.Name)
}
