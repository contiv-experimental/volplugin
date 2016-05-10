package test

import (
	"encoding/json"
	"fmt"
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
const BackendName = "test"

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
	BaseMountPath string
	CreatedMap    map[string]bool
	MountedMap    map[string]bool
}

// New constructs an empty *Driver
func New() *Driver {
	if gNullDriver == nil {
		gNullDriver = &Driver{
			CreatedMap: make(map[string]bool),
			MountedMap: make(map[string]bool),
		}
	}

	return gNullDriver
}

// NewMountDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewMountDriver(BaseMountPath string) (storage.MountDriver, error) {
	log.Infof("In %s", getFunctionName())
	driver := New()
	driver.BaseMountPath = BaseMountPath

	return driver, nil
}

// NewSnapshotDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewSnapshotDriver() (storage.SnapshotDriver, error) {
	return New(), nil
}

// NewCRUDDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewCRUDDriver() (storage.CRUDDriver, error) {
	return New(), nil
}

func (d *Driver) logStat(funcName string) {
	log.Infof("In %q:", funcName)
	content, err := json.MarshalIndent(d, "\t", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println("\t" + string(content))
}

// Name returns the test backend string
func (d *Driver) Name() string {
	d.logStat(getFunctionName())
	return BackendName
}

// Create records created volume.
func (d *Driver) Create(do storage.DriverOptions) error {
	d.logStat(getFunctionName())
	d.CreatedMap[filepath.Join(d.BaseMountPath, do.Volume.Params["pool"], do.Volume.Name)] = true
	return nil
}

// Format is a noop.
func (d *Driver) Format(do storage.DriverOptions) error {
	d.logStat(getFunctionName())
	return nil
}

// Destroy deletes volume entry
func (d *Driver) Destroy(do storage.DriverOptions) error {
	d.logStat(getFunctionName())
	delete(d.CreatedMap, filepath.Join(d.BaseMountPath, do.Volume.Params["pool"], do.Volume.Name))
	return nil
}

// List Volumes returns empty list.
func (d *Driver) List(lo storage.ListOptions) ([]storage.Volume, error) {
	d.logStat(getFunctionName())
	log.Infof("In %q", getFunctionName())
	return []storage.Volume{}, nil
}

// Mount records mounted volume
func (d *Driver) Mount(do storage.DriverOptions) (*storage.Mount, error) {
	d.logStat(getFunctionName())
	if err := os.MkdirAll(d.BaseMountPath, 0700); err != nil && !os.IsExist(err) {
		return nil, errored.Errorf("error creating %q directory: %v", d.BaseMountPath, err)
	}

	volumePath := filepath.Join(d.BaseMountPath, do.Volume.Params["pool"], do.Volume.Name)
	if err := os.MkdirAll(volumePath, 0700); err != nil && !os.IsExist(err) {
		return nil, errored.Errorf("error creating %q directory: %v", volumePath, err)
	}
	d.MountedMap[filepath.Join(d.BaseMountPath, do.Volume.Params["pool"], do.Volume.Name)] = true
	return nil, nil
}

// Unmount deletes mounted entry.
func (d *Driver) Unmount(do storage.DriverOptions) error {
	d.logStat(getFunctionName())
	delete(d.MountedMap, filepath.Join(d.BaseMountPath, do.Volume.Params["pool"], do.Volume.Name))
	volumePath := filepath.Join(d.BaseMountPath, do.Volume.Params["pool"], do.Volume.Name)
	if err := os.RemoveAll(volumePath); err != nil {
		return errored.Errorf("error deleting %q directory: %v", volumePath, err)
	}
	return nil
}

// Exists returns false always.
func (d *Driver) Exists(do storage.DriverOptions) (bool, error) {
	d.logStat(getFunctionName())
	if _, ok := d.CreatedMap[filepath.Join(d.BaseMountPath, do.Volume.Params["pool"], do.Volume.Name)]; !ok {
		return false, errored.Errorf("volume not CreatedMap: %#v", d.CreatedMap)
	}
	return true, nil
}

// CreateSnapshot is a noop.
func (d *Driver) CreateSnapshot(s string, do storage.DriverOptions) error {
	d.logStat(getFunctionName())
	return nil
}

// RemoveSnapshot is a noop.
func (d *Driver) RemoveSnapshot(s string, do storage.DriverOptions) error {
	d.logStat(getFunctionName())
	return nil
}

// CopySnapshot is a noop.
func (d *Driver) CopySnapshot(do storage.DriverOptions, s, s2 string) error {
	d.logStat(getFunctionName())
	return nil
}

// ListSnapshots returns an empty list.
func (d *Driver) ListSnapshots(do storage.DriverOptions) ([]string, error) {
	d.logStat(getFunctionName())
	return []string{}, nil
}

// Mounted returns an empty list.
func (d *Driver) Mounted(t time.Duration) ([]*storage.Mount, error) {
	d.logStat(getFunctionName())
	return []*storage.Mount{}, nil
}

// MountPath describes the path at which the volume should be mounted.
func (d *Driver) MountPath(do storage.DriverOptions) (string, error) {
	d.logStat(getFunctionName())
	return filepath.Join(d.BaseMountPath, do.Volume.Params["pool"], do.Volume.Name), nil
}

// Validate returns nil.
func (d *Driver) Validate(storage.DriverOptions) error {
	return nil
}
