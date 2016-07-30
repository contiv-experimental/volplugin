package backend

import (
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend/ceph"
	"github.com/contiv/volplugin/storage/backend/nfs"
)

// DriverTypes
var (
	CRUD      = "crud"
	Mount     = "mount"
	Snapshot  = "snapshot"
	MountPath = "/mnt"
)

// MountDrivers is the map of string to storage.MountDriver.
var MountDrivers = map[string]func(string) (storage.MountDriver, error){
	ceph.BackendName: ceph.NewMountDriver,
	nfs.BackendName:  nfs.NewMountDriver,
}

// CRUDDrivers is the map of string to storage.CRUDDriver.
var CRUDDrivers = map[string]func() (storage.CRUDDriver, error){
	ceph.BackendName: ceph.NewCRUDDriver,
}

// SnapshotDrivers is the map of string to storage.SnapshotDriver.
var SnapshotDrivers = map[string]func() (storage.SnapshotDriver, error){
	ceph.BackendName: ceph.NewSnapshotDriver,
}

// NewMountDriver instantiates and return a mount driver instance of the
// specified type
func NewMountDriver(backend, mountpath string) (storage.MountDriver, error) {
	f, ok := MountDrivers[backend]
	if !ok {
		return nil, errored.Errorf("invalid mount driver backend: %q", backend)
	}

	if mountpath == "" {
		return nil, errored.Errorf("mount path not specified, cannot continue")
	}

	return f(mountpath)
}

// NewCRUDDriver instantiates a CRUD Driver.
func NewCRUDDriver(backend string) (storage.CRUDDriver, error) {
	f, ok := CRUDDrivers[backend]
	if !ok {
		return nil, errored.Errorf("invalid CRUD driver backend: %q", backend)
	}

	return f()
}

// NewSnapshotDriver creates a SnapshotDriver based on the backend name.
func NewSnapshotDriver(backend string) (storage.SnapshotDriver, error) {
	f, ok := SnapshotDrivers[backend]
	if !ok {
		return nil, errored.Errorf("invalid snapshot driver backend: %q", backend)
	}

	return f()
}

// NewDriver creates a driver based on driverType
func NewDriver(backend string, driverType string, mountPath string, do *storage.DriverOptions) error {
	if backend != "" && do != nil {
		switch driverType {
		case CRUD:
			if crud, err := NewCRUDDriver(backend); err != nil {
				return err
			} else if err := crud.Validate(do); err != nil {
				return err
			}
		case Mount:
			if mount, err := NewMountDriver(backend, mountPath); err != nil {
				return err
			} else if err := mount.Validate(do); err != nil {
				return err
			}
		case Snapshot:
			if snapshot, err := NewSnapshotDriver(backend); err != nil {
				return err
			} else if err := snapshot.Validate(do); err != nil {
				return err
			}
		default:
			return errored.Errorf("Invalid driver type: %q", driverType)
		}
	} else {
		return errored.Errorf("Empty backend or driver options")
	}

	return nil // On successful driver creation
}
