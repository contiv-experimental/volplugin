// Package librbd provides functionality to interact with Ceph RADOS block device (RBD)
// subsystem and underlying kernel support. It requires the librados and librbd
// libraries to function.
//
// No operations are handled under lock; this is a deliberate design decision
// that allows you to implement the locks in the distributed fashion of your
// choice.
package librbd

// #cgo LDFLAGS: -lrbd -lrados
// #include <rados/librados.h>
// #include <rbd/librbd.h>
// #include <stdlib.h>
// rbd_image_t*  make_image() {
//   return malloc(sizeof(rbd_image_t));
// }
//
import "C"
import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"
)

var (
	rbdBusPath    = "/sys/bus/rbd"
	rbdDevicePath = path.Join(rbdBusPath, "devices")
	rbdDev        = "/dev/rbd"
)

// RBDConfig provides a JSON representation of some Ceph configuration elements
// that are vital to librbd's use.  librados does not support nested
// configuration; we may sometimes be stuck with this (and in the ansible, are)
// so we need a back up plan on how to manage configuration. This is it. See
// ReadConfig.
type RBDConfig struct {
	MonitorIP string `json:"monitor_ip"`
	UserName  string `json:"username"`
	Secret    string `json:"secret"`
}

// Pool is a unit of storage composing of many images.
type Pool struct {
	ioctx     C.rados_ioctx_t
	cluster   C.rados_t
	poolName  string
	rbdConfig RBDConfig
}

// Version returns the version of librbd.
func Version() (int, int, int) {
	var major, minor, extra C.int

	C.rbd_version(&major, &minor, &extra)
	return int(major), int(minor), int(extra)
}

// ReadConfig parses a RBDConfig and returns it.
func ReadConfig(path string) (RBDConfig, error) {
	config := RBDConfig{}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(content, &config)

	return config, err
}

func getRados(username string) (C.rados_t, error) {
	var cluster C.rados_t

	str := C.CString(username)
	defer C.free(unsafe.Pointer(str))

	if i := C.rados_create(&cluster, str); i < 0 {
		return nil, strerror(i)
	}

	if i := C.rados_conf_read_file(cluster, nil); i < 0 {
		return nil, strerror(i)
	}

	if i := C.rados_connect(cluster); i != 0 {
		return nil, strerror(i)
	}

	return cluster, nil
}

func freePool(pool *Pool) {
	C.rados_ioctx_destroy(pool.ioctx)
	C.rados_shutdown(pool.cluster)
}

// GetPool instantiates a Pool object from librados. It must be able to
// authenticate to ceph through normal (e.g., CLI) means to perform this
// operation. The RBDConfig is used to supply parts of the configuration which
// cannot be necessarily parsed by the C versions of librados and librbd. This
// struct will need to be populated; if you want a way to fill it from JSON,
// see ReadConfig.
func GetPool(config RBDConfig, poolName string) (*Pool, error) {
	var err error

	pool := &Pool{poolName: poolName, rbdConfig: config}

	str := C.CString(poolName)
	defer C.free(unsafe.Pointer(str))

	pool.cluster, err = getRados(config.UserName)
	if err != nil {
		return nil, err
	}

	if i := C.rados_ioctx_create(pool.cluster, str, &pool.ioctx); i < 0 {
		return nil, strerror(i)
	}

	runtime.SetFinalizer(pool, freePool)

	return pool, nil
}

// CreateImage creates a rbd image for the pool. Given a name and size in
// bytes, it will return any error if there was a problem creating the
// image.
func (p *Pool) CreateImage(name string, size uint64) error {
	str := C.CString(name)
	defer C.free(unsafe.Pointer(str))

	order := C.int(0)

	if i := C.rbd_create(p.ioctx, str, C.uint64_t(size), &order); i < 0 {
		return strerror(i)
	}

	return nil
}

// RemoveImage removes an image named by the string. Returns an error on failure.
func (p *Pool) RemoveImage(name string) error {
	str := C.CString(name)
	defer C.free(unsafe.Pointer(str))

	if i := C.rbd_remove(p.ioctx, str); i < 0 {
		return strerror(i)
	}

	return nil
}

// List all the images for the pool.
func (p *Pool) List() ([]string, error) {
	list := C.CString("")
	defer func() {
		C.free(unsafe.Pointer(list))
	}()

	// FIXME number of entries, but it's an undocumented call so I don't know for sure
	sizeT := C.size_t(1024 * 1024)

	var i C.int
	if i = C.rbd_list(p.ioctx, list, &sizeT); i < 0 {
		return nil, strerror(i)
	}

	// the returned string is multiple null terminated strings with a double null
	// at the end. Hence GoStringN.
	items := strings.Split(C.GoStringN(list, i), string([]byte{0}))
	return items[:len(items)-1], nil
}

func (p *Pool) wrapOpen(imageName string, action func(*C.rbd_image_t) error) error {
	image := C.make_image()
	imageStr := C.CString(imageName)
	defer func() {
		C.free(unsafe.Pointer(image))
		C.free(unsafe.Pointer(imageStr))
	}()

	if i, err := C.rbd_open(p.ioctx, imageStr, image, nil); err != nil || i < 0 {
		if i < 0 {
			err = strerror(i)
		}

		return fmt.Errorf("Error creating snapshot: %v", err)
	}

	defer func() error {
		if i, err := C.rbd_close(*image); err != nil || i < 0 {
			if i < 0 {
				err = strerror(i)
			}

			return fmt.Errorf("Error creating snapshot: %v", err)
		}

		return nil
	}()

	if err := action(image); err != nil {
		return err
	}

	return nil
}

// ListSnapshots yields a list of the snapshots for the given interface. max
// will yield a maximum of N items.
func (p *Pool) ListSnapshots(imageName string, max int) ([]string, error) {
	snapInfo := make([]C.rbd_snap_info_t, max)
	list := []string{}
	cMax := C.int(max)

	action := func(image *C.rbd_image_t) error {
		if i, err := C.rbd_snap_list(*image, &snapInfo[0], &cMax); err != nil || i < 0 {
			return strerror(i)
		}

		for i := 0; i < int(cMax); i++ {
			if C.GoString(snapInfo[i].name) == "" {
				return nil
			}

			list = append(list, C.GoString(snapInfo[i].name))
		}

		return nil
	}

	if err := p.wrapOpen(imageName, action); err != nil {
		return nil, err
	}

	return list, nil
}

// CreateSnapshot creates a named snapshot for the image provided.
func (p *Pool) CreateSnapshot(imageName, snapshotName string) error {
	snapshotStr := C.CString(snapshotName)
	defer func() {
		C.free(unsafe.Pointer(snapshotStr))
	}()

	action := func(image *C.rbd_image_t) error {
		if i, err := C.rbd_snap_create(*image, snapshotStr); err != nil || i < 0 {
			if i < 0 {
				err = strerror(i)
			}

			return fmt.Errorf("Error creating snapshot: %v", err)
		}

		return nil
	}

	if err := p.wrapOpen(imageName, action); err != nil {
		return err
	}

	return nil
}

// RemoveSnapshot deletes a named snapshot for the image provided.
func (p *Pool) RemoveSnapshot(imageName, snapshotName string) error {
	snapshotStr := C.CString(snapshotName)
	defer func() {
		C.free(unsafe.Pointer(snapshotStr))
	}()

	action := func(image *C.rbd_image_t) error {
		if i, err := C.rbd_snap_remove(*image, snapshotStr); err != nil || i < 0 {
			if i < 0 {
				err = strerror(i)
			}

			return fmt.Errorf("Error creating snapshot: %v", err)
		}

		return nil
	}

	if err := p.wrapOpen(imageName, action); err != nil {
		return err
	}

	return nil
}

// MapDevice maps an image to a device on the host. Returns the device path and
// any errors. On error, the device path will be blank.
func (p *Pool) MapDevice(imageName string) (string, error) {
	if str, err := p.findDevice(imageName); err == nil {
		return str, nil
	}

	addF, err := os.OpenFile(filepath.Join(rbdBusPath, "add"), os.O_WRONLY, 0200)
	if err != nil {
		return "", err
	}

	defer addF.Close()

	output := fmt.Sprintf("%s name=%s,secret=%s %s %s", p.rbdConfig.MonitorIP, p.rbdConfig.UserName, p.rbdConfig.Secret, p.poolName, imageName)

	if _, err := addF.Write([]byte(output)); err != nil {
		if err != nil {
			modprobeRBD()
			if _, err := addF.Write([]byte(output)); err != nil {
				return "", err
			}
		}
	}

	return p.findDevice(imageName)
}

// UnmapDevice removes a RBD device. Given an image name, it will locate the
// device and unmap it. Returns an error on any failure.
func (p *Pool) UnmapDevice(imageName string) error {
	devName, err := p.findDevice(imageName)
	if err != nil {
		return err
	}

	devNum := strings.TrimPrefix(devName, rbdDev)

	if _, err := os.Stat(devName); err != nil {
		println("here")
		return os.ErrNotExist
	}

	removePath := filepath.Join(rbdBusPath, "remove")

	if _, err := os.Stat(removePath); err != nil {
		return fmt.Errorf("Can't locate remove file: %v", err)
	}

	remF, err := os.OpenFile(removePath, os.O_WRONLY, 0200)
	if err != nil {
		return fmt.Errorf("Error writing to remove file: %v", err)
	}

	defer remF.Close()

	var success bool

	for {
		if _, err := remF.Write([]byte(devNum)); err != nil {
			if success {
				return nil
			}
		}

		success = true

		time.Sleep(100 * time.Millisecond)
	}
}
