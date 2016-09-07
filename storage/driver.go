package storage

import (
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
)

var (
	// ErrVolumeExist indicates that a volume already exists.
	ErrVolumeExist = errors.New("Volume already exists")
)

// DriverParams are parameters that relate directly to the location of the storage.
type DriverParams map[string]interface{}

// A Mount is the resulting attributes of a Mount or Unmount operation.
type Mount struct {
	Device   string
	Path     string
	DevMajor uint
	DevMinor uint
	Volume   Volume
}

// FSOptions encapsulates the parameters to create and manipulate filesystems.
type FSOptions struct {
	Type          string
	CreateCommand string
}

// DriverOptions are options frequently passed as the keystone for operations.
// See Driver for more information.
type DriverOptions struct {
	Source    string
	Volume    Volume
	FSOptions FSOptions
	Timeout   time.Duration
	Options   map[string]string
}

// ListOptions is a set of parameters used for the List operation of Driver.
type ListOptions struct {
	Params DriverParams
}

// Volume is the basic representation of a volume name and its parameters.
type Volume struct {
	Name   string
	Size   uint64
	Params DriverParams
}

// NamedDriver is a named driver and has a method called Name()
type NamedDriver interface {
	// Name returns the string associated with the storage backed of the driver
	Name() string
}

// ValidatingDriver implements Validate() against storage.DriverOptions.
type ValidatingDriver interface {
	Validate(*DriverOptions) error
}

// MountDriver mounts volumes.
type MountDriver interface {
	NamedDriver
	ValidatingDriver

	// Mount a Volume
	Mount(DriverOptions) (*Mount, error)

	// Unmount a volume
	Unmount(DriverOptions) error

	// Mounted shows any volumes that belong to volplugin on the host, in
	// their native representation. They yield a *Mount.
	Mounted(time.Duration) ([]*Mount, error)

	// MountPath describes the path at which the volume should be mounted.
	MountPath(DriverOptions) (string, error)
}

// CRUDDriver performs CRUD operations.
type CRUDDriver interface {
	NamedDriver
	ValidatingDriver

	// Create a volume.
	Create(DriverOptions) error

	// Format a volume.
	Format(DriverOptions) error

	// Destroy a volume.
	Destroy(DriverOptions) error

	// List Volumes. May be scoped by storage parameters or other data.
	List(ListOptions) ([]Volume, error)

	// Exists returns true if a volume exists. Otherwise, it returns false.
	Exists(DriverOptions) (bool, error)
}

// SnapshotDriver manages snapshots.
type SnapshotDriver interface {
	NamedDriver
	ValidatingDriver

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
}

// Validate validates driver options to ensure they are compatible with all
// storage drivers.
func (do *DriverOptions) Validate() error {
	if do.Timeout == 0 {
		return errored.Errorf("Missing timeout in storage driver")
	}

	return do.Volume.Validate()
}

// Validate validates volume options to ensure they are compatible with all
// storage drivers.
func (v Volume) Validate() error {
	if v.Name == "" {
		return errored.Errorf("Name is missing in storage driver")
	}

	if v.Params == nil {
		return errored.Errorf("Params are nil in storage driver")
	}

	return nil
}

// Get retrieves the value of attribute `attrName` from DriverParams
func (dp DriverParams) Get(attrName string, result interface{}) error {
	if _, ok := dp[attrName]; !ok { // attrName not found in DriverParams
		return nil
	}

	expectedType := reflect.TypeOf(result)
	if expectedType == nil {
		return errored.Errorf("Cannot use <nil> as pointer type")
	} else if expectedType.Kind() != reflect.Ptr {
		return errored.Errorf("Cannot reference a non-pointer type %q", expectedType.Kind())
	}

	expectedKind := expectedType.Elem().Kind()

	actualType := reflect.TypeOf(dp[attrName])
	if actualType == nil { // <nil> types
		return errored.Errorf("Expected %q; Cannot use <nil> value for driver.%q", expectedKind, attrName)
	}

	actualKind := actualType.Kind()

	value := reflect.ValueOf(dp[attrName]) // returns zero-value for <nil> value
	switch value.Kind() {
	case reflect.String, reflect.Int, reflect.Float64, reflect.Float32, reflect.Bool:
		if actualKind != expectedKind {
			return errored.Errorf("Cannot use %q as type %q", expectedKind, actualKind)
		}
		reflect.ValueOf(result).Elem().Set(value) // succeeds only if the types match
	case reflect.Map:
		if expectedKind != reflect.Map && expectedKind != reflect.Struct {
			return errored.Errorf("Expected map or struct; Cannot use %q as type %q", expectedKind, actualKind)
		}
		content, err := json.Marshal(value.Interface())
		if err != nil {
			return err
		}

		if err := json.Unmarshal(content, result); err != nil {
			return err
		}
	default:
		logrus.Info("Unknown type %q", value.Kind())
	}

	return nil
}
