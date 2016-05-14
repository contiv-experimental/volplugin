// +build nope

package null

import "github.com/contiv/volplugin/storage"

// Driver for null operations. Does nothing.
type Driver struct{}

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
