package storage

import (
	"errors"
	"time"
)

var (
	// ErrVolumeExist indicates that a volume already exists.
	ErrVolumeExist = errors.New("Volume already exists")
)

// Params are parameters that relate directly to the location of the storage.
type Params map[string]string

// A Mount is the resulting attributes of a Mount or Unmount operation.
type Mount struct {
	Device   string
	Path     string
	DevMajor uint
	DevMinor uint
	Volume
}

// FSOptions encapsulates the parameters to create and manipulate filesystems.
type FSOptions struct {
	Type          string
	CreateCommand string
}

// DriverOptions are options frequently passed as the keystone for operations.
// See Driver for more information.
type DriverOptions struct {
	Volume
	FSOptions
	Timeout time.Duration
	Options map[string]string
}

// ListOptions is a set of parameters used for the List operation of Driver.
type ListOptions struct {
	Params
}

// Volume is the basic representation of a volume name and its parameters.
type Volume struct {
	Name string
	Size uint64
	Params
}

// Driver is a full driver that implements a storage function available to
// volplugin. Drivers are called anywhere filesystem operations are necessary.
// Consumers should do this through the NewStorage call and associated member
// functions.
type Driver interface {
	// Name returns the string associated with the storage backed of the driver
	Name() string

	// Create a volume.
	Create(DriverOptions) error

	// Format a volume.
	Format(DriverOptions) error

	// Destroy a volume.
	Destroy(DriverOptions) error

	// List Volumes. May be scoped by storage parameters or other data.
	List(ListOptions) ([]Volume, error)

	// Mount a Volume
	Mount(DriverOptions) (*Mount, error)

	// Unmount a volume
	Unmount(DriverOptions) error

	// Exists returns true if a volume exists. Otherwise, it returns false.
	Exists(DriverOptions) (bool, error)

	// CreateSnapshot creates a named snapshot for the volume. Any error will be returned.
	CreateSnapshot(string, DriverOptions) error

	// RemoveSnapshot removes a named snapshot for the volume. Any error will be returned.
	RemoveSnapshot(string, DriverOptions) error

	// ListSnapshots returns an array of snapshot names provided a maximum number
	// of snapshots to be returned. Any error will be returned.
	ListSnapshots(DriverOptions) ([]string, error)

	// CopySnapshot copies a snapshot into a new volume. Takes a DriverOptions,
	// snap and volume name (string). Returns error on failure.
	CopySnapshot(DriverOptions, string, string) error

	// Mounted shows any volumes that belong to volplugin on the host, in
	// their native representation. They yield a *Mount.
	Mounted(time.Duration) ([]*Mount, error)

	// InternalName translates a volplugin `tenant/volume` name to an internal
	// name suitable for the driver. Yields an error if impossible.
	InternalName(string) (string, error)

	// InternalNameToVolpluginName translates an internal name to a volplugin
	// `tenant/volume` syntax name.
	InternalNameToVolpluginName(string) string

	// MountPath describes the path at which the volume should be mounted.
	MountPath(DriverOptions) string
}
