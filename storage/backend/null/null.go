package null

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/storage"
)

// BackendName is string for no-op storage backend
const BackendName = "null"

// gNullDriver is the singleton driver instace to keep all state in memory
var gNullDriver *Driver

func getFunctionName() string {
	if pc, _, _, ok := runtime.Caller(1); ok {
		f := strings.Split(runtime.FuncForPC(pc).Name(), "/")
		return f[len(f)-1]
	}
	return "unknown"
}

// Driver implements a no-op storage driver for volplugin.
//
// This is intended for regressing volplugin
type Driver struct {
	mountpath string
	created   map[string]bool
	mounted   map[string]bool
}

// NewDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewDriver(mountpath string) storage.Driver {
	log.Infof("In %s", getFunctionName())
	if gNullDriver != nil {
		return gNullDriver
	}
	gNullDriver = &Driver{
		mountpath: mountpath,
		created:   make(map[string]bool),
		mounted:   make(map[string]bool),
	}
	return gNullDriver
}

// Name returns the null backend string
func (d *Driver) Name() string {
	log.Infof("In %q", getFunctionName())
	return BackendName
}

// Create records created volume.
func (d *Driver) Create(do storage.DriverOptions) error {
	log.Infof("In %q", getFunctionName())
	d.created[filepath.Join(d.mountpath, do.Volume.Params["pool"], do.Volume.Name)] = true
	return nil
}

// Format is a noop.
func (d *Driver) Format(do storage.DriverOptions) error {
	log.Infof("In %q", getFunctionName())
	return nil
}

// Destroy deletes volume entry
func (d *Driver) Destroy(do storage.DriverOptions) error {
	log.Infof("In %q", getFunctionName())
	delete(d.created, filepath.Join(d.mountpath, do.Volume.Params["pool"], do.Volume.Name))
	return nil
}

// List Volumes returns empty list.
func (d *Driver) List(lo storage.ListOptions) ([]storage.Volume, error) {
	log.Infof("In %q", getFunctionName())
	return []storage.Volume{}, nil
}

// Mount records mounted volume
func (d *Driver) Mount(do storage.DriverOptions) (*storage.Mount, error) {
	log.Infof("In %q", getFunctionName())
	if err := os.MkdirAll(d.mountpath, 0700); err != nil && !os.IsExist(err) {
		return nil, errored.Errorf("error creating %q directory: %v", d.mountpath, err)
	}

	volumePath := filepath.Join(d.mountpath, do.Volume.Params["pool"], do.Volume.Name)
	if err := os.MkdirAll(volumePath, 0700); err != nil && !os.IsExist(err) {
		return nil, errored.Errorf("error creating %q directory: %v", volumePath, err)
	}
	d.mounted[filepath.Join(d.mountpath, do.Volume.Params["pool"], do.Volume.Name)] = true
	return nil, nil
}

// Unmount deleted mounted volume entry.
func (d *Driver) Unmount(do storage.DriverOptions) error {
	log.Infof("In %q", getFunctionName())
	delete(d.mounted, filepath.Join(d.mountpath, do.Volume.Params["pool"], do.Volume.Name))
	volumePath := filepath.Join(d.mountpath, do.Volume.Params["pool"], do.Volume.Name)
	if err := os.RemoveAll(volumePath); err != nil {
		return errored.Errorf("error deleting %q directory: %v", volumePath, err)
	}
	return nil
}

// Exists returns false always.
func (d *Driver) Exists(do storage.DriverOptions) (bool, error) {
	log.Infof("In %q", getFunctionName())
	if _, ok := d.created[filepath.Join(d.mountpath, do.Volume.Params["pool"], do.Volume.Name)]; !ok {
		return false, errored.Errorf("volume not created")
	}
	return true, nil
}

// CreateSnapshot is a noop.
func (d *Driver) CreateSnapshot(s string, do storage.DriverOptions) error {
	log.Infof("In %q", getFunctionName())
	return nil
}

// RemoveSnapshot is a noop.
func (d *Driver) RemoveSnapshot(s string, do storage.DriverOptions) error {
	log.Infof("In %q", getFunctionName())
	return nil
}

// CopySnapshot is a noop.
func (d *Driver) CopySnapshot(do storage.DriverOptions, s, s2 string) error {
	log.Infof("In %q", getFunctionName())
	return nil
}

// ListSnapshots returns an empty list.
func (d *Driver) ListSnapshots(do storage.DriverOptions) ([]string, error) {
	log.Infof("In %q", getFunctionName())
	return []string{}, nil
}

// Mounted retuns an empty list.
func (d *Driver) Mounted(t time.Duration) ([]*storage.Mount, error) {
	log.Infof("In %q", getFunctionName())
	return []*storage.Mount{}, nil
}

// InternalName returns the passed string as is.
func (d *Driver) InternalName(s string) (string, error) {
	log.Infof("In %q", getFunctionName())
	return s, nil
}

// InternalNameToVolpluginName returns the passed string as is.
func (d *Driver) InternalNameToVolpluginName(s string) string {
	log.Infof("In %q", getFunctionName())
	return s
}

// MountPath describes the path at which the volume should be mounted.
func (d *Driver) MountPath(do storage.DriverOptions) string {
	log.Infof("In %q", getFunctionName())
	return filepath.Join(d.mountpath, do.Volume.Params["pool"], do.Volume.Name)
}
