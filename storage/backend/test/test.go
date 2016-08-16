// +build nope

package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/storage"
	"github.com/coreos/etcd/client"
)

// BackendName is string for no-op storage backend
const BackendName = "test"

const prefix = "/testdriver"

var (
	volumesPrefix = path.Join(prefix, "volume")
	mountedPrefix = path.Join(prefix, "mounted")
)

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

	client client.KeysAPI
}

// New constructs an empty *Driver
func New() *Driver {
	etcdClient, err := client.New(client.Config{Endpoints: []string{"http://localhost:2379"}})
	if err != nil {
		panic(err)
	}

	if gNullDriver == nil {
		gNullDriver = &Driver{
			CreatedMap: make(map[string]bool),
			MountedMap: make(map[string]bool),
			client:     client.NewKeysAPI(etcdClient),
		}
	}

	for _, p := range []string{prefix, volumesPrefix, mountedPrefix} {
		gNullDriver.client.Set(context.Background(), p, "", &client.SetOptions{Dir: true})
	}

	return gNullDriver
}

// NewMountDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewMountDriver(BaseMountPath string) (storage.MountDriver, error) {
	logrus.Infof("In %s", getFunctionName())
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
	logrus.Infof("In %q:", funcName)
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

	content, err := json.Marshal(do)
	if err != nil {
		return err
	}

	_, err = d.client.Set(context.Background(), path.Join(volumesPrefix, do.Volume.Name), string(content), &client.SetOptions{PrevExist: client.PrevNoExist})
	return err
}

// Format is a noop.
func (d *Driver) Format(do storage.DriverOptions) error {
	d.logStat(getFunctionName())
	return nil
}

// Destroy deletes volume entry
func (d *Driver) Destroy(do storage.DriverOptions) error {
	d.logStat(getFunctionName())
	logrus.Info(path.Join(volumesPrefix, do.Volume.Name))
	_, err := d.client.Delete(context.Background(), path.Join(volumesPrefix, do.Volume.Name), nil)
	return err
}

// List Volumes list of volumes in etcd.
func (d *Driver) List(lo storage.ListOptions) ([]storage.Volume, error) {
	d.logStat(getFunctionName())
	logrus.Infof("In %q", getFunctionName())
	nodes, err := d.client.Get(context.Background(), volumesPrefix, &client.GetOptions{Recursive: true})
	if err != nil {
		return nil, err
	}

	volumes := []storage.Volume{}

	for _, n := range nodes.Node.Nodes { // inner nodes
		for _, node := range n.Nodes {
			key := strings.TrimPrefix(node.Key, volumesPrefix)
			volumes = append(volumes, storage.Volume{Name: key})
		}
	}

	return volumes, nil
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

	mount := &storage.Mount{
		Path:   volumePath,
		Volume: do.Volume,
	}

	content, err := json.Marshal(mount)
	if err != nil {
		return nil, err
	}

	_, err = d.client.Set(context.Background(), path.Join(mountedPrefix, do.Volume.Name), string(content), &client.SetOptions{PrevExist: client.PrevNoExist})
	logrus.Infof("%v %v", path.Join(mountedPrefix, do.Volume.Name), err)
	if err != nil {
		return nil, err
	}

	return mount, nil
}

// Unmount deletes mounted entry.
func (d *Driver) Unmount(do storage.DriverOptions) error {
	d.logStat(getFunctionName())
	_, err := d.client.Delete(context.Background(), path.Join(mountedPrefix, do.Volume.Name), nil)
	logrus.Infof("%v %v", path.Join(mountedPrefix, do.Volume.Name), err)
	return err
}

// Exists returns false always.
func (d *Driver) Exists(do storage.DriverOptions) (bool, error) {
	d.logStat(getFunctionName())

	_, err := d.client.Get(context.Background(), path.Join(volumesPrefix, do.Volume.Name), nil)
	if err != nil {
		if _, ok := err.(client.Error); ok && err.(client.Error).Code == client.ErrorCodeKeyNotFound {
			return false, nil
		}

		return false, errored.Errorf("Error retriving key %q", path.Join(mountedPrefix, do.Volume.Name)).Combine(err)
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
	nodes, err := d.client.Get(context.Background(), mountedPrefix, &client.GetOptions{Recursive: true})
	if err != nil {
		return nil, err
	}

	mounts := []*storage.Mount{}
	for _, n := range nodes.Node.Nodes {
		for _, node := range n.Nodes {
			key := strings.TrimPrefix(node.Key, mountedPrefix)
			do := storage.DriverOptions{Volume: storage.Volume{Name: key}}
			mp, err := d.MountPath(do)
			if err != nil {
				return nil, err
			}

			mounts = append(mounts, &storage.Mount{Volume: do.Volume, Path: mp})
		}
	}

	return mounts, nil
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
