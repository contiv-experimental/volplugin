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
var MountDrivers = map[string]func(string, map[string]interface{}) (storage.MountDriver, error){
	ceph.BackendName: ceph.NewMountDriver,
	nfs.BackendName:  nfs.NewMountDriver,
}

// CRUDDrivers is the map of string to storage.CRUDDriver.
var CRUDDrivers = map[string]func(map[string]interface{}) (storage.CRUDDriver, error){
	ceph.BackendName: ceph.NewCRUDDriver,
}

// SnapshotDrivers is the map of string to storage.SnapshotDriver.
var SnapshotDrivers = map[string]func() (storage.SnapshotDriver, error){
	ceph.BackendName: ceph.NewSnapshotDriver,
}

// NewMountDriver instantiates and return a mount driver instance of the
// specified type
func NewMountDriver(backend, mountpath string, dOptions map[string]interface{}) (storage.MountDriver, error) {
	f, ok := MountDrivers[backend]
	if !ok {
		return nil, errored.Errorf("invalid mount driver backend: %q", backend)
	}

	if mountpath == "" {
		return nil, errored.Errorf("mount path not specified, cannot continue")
	}

	return f(mountpath, dOptions)
}

// NewCRUDDriver instantiates a CRUD Driver.
func NewCRUDDriver(backend string, dOptions map[string]interface{}) (storage.CRUDDriver, error) {
	f, ok := CRUDDrivers[backend]
	if !ok {
		return nil, errored.Errorf("invalid CRUD driver backend: %q", backend)
	}

	return f(dOptions)
}

// NewSnapshotDriver creates a SnapshotDriver based on the backend name.
func NewSnapshotDriver(backend string) (storage.SnapshotDriver, error) {
	f, ok := SnapshotDrivers[backend]
	if !ok {
		return nil, errored.Errorf("invalid snapshot driver backend: %q", backend)
	}

	return f()
}
