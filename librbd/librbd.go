package librbd

// #cgo LDFLAGS: -lrbd -lrados
// #include <rados/librados.h>
// #include <rbd/librbd.h>
// #include <stdlib.h>
// #include <errno.h>
// #include <string.h>
import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"
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
	ioctx    C.rados_ioctx_t
	cluster  C.rados_t
	poolName string
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

func strerror(i C.int) error {
	return errors.New(C.GoString(C.strerror(-i)))
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
// operation.
func GetPool(username, poolName string) (*Pool, error) {
	var err error

	pool := &Pool{poolName: poolName}

	str := C.CString(poolName)
	defer C.free(unsafe.Pointer(str))

	pool.cluster, err = getRados(username)
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
	size_t := C.size_t(1024 * 1024)

	var i C.int
	if i = C.rbd_list(p.ioctx, list, &size_t); i < 0 {
		return nil, strerror(i)
	}

	// the returned string is multiple null terminated strings with a double null
	// at the end. Hence GoStringN.
	items := strings.Split(C.GoStringN(list, i), string([]byte{0}))
	return items[:len(items)-1], nil
}

func (p *Pool) findDevice(imageName string) (string, error) {
	fi, err := ioutil.ReadDir("/sys/bus/rbd/devices")
	if err != nil && err != os.ErrNotExist {
		return "", err
	} else if err == os.ErrNotExist {
		return "", fmt.Errorf("Could not locate devices directory")
	}

	for _, f := range fi {
		namePath := filepath.Join("/sys/bus/rbd/devices", f.Name(), "name")
		content, err := ioutil.ReadFile(namePath)
		if err != nil {
			return "", err
		}

		if strings.TrimSpace(string(content)) == imageName {
			poolPath := filepath.Join("/sys/bus/rbd/devices", f.Name(), "pool")
			content, err := ioutil.ReadFile(poolPath)
			if err != nil {
				return "", err
			}

			if strings.TrimSpace(string(content)) == p.poolName {
				if _, err := os.Stat("/dev/rbd" + f.Name()); err != nil {
					return "", err
				}

				return "/dev/rbd" + f.Name(), nil
			}
		}
	}

	return "", os.ErrNotExist
}

func modprobeRBD() error {
	return exec.Command("modprobe", "rbd").Run()
}

// MapDevice maps an image to a device on the host. Returns the device path and
// any errors. On error, the device path will be blank.
func (p *Pool) MapDevice(monIP, username, secret, imageName string) (string, error) {
	if str, err := p.findDevice(imageName); err == nil {
		return str, nil
	}

	addF, err := os.OpenFile("/sys/bus/rbd/add", os.O_WRONLY, 0200)
	if err != nil {
		return "", err
	}

	defer addF.Close()

	output := fmt.Sprintf("%s name=%s,secret=%s rbd %s", monIP, username, secret, imageName)

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

	devNum := strings.TrimPrefix(devName, "/dev/rbd")

	if _, err := os.Stat(devName); err != nil {
		println("here")
		return os.ErrNotExist
	}

	if _, err := os.Stat("/sys/bus/rbd/remove"); err != nil {
		return fmt.Errorf("Can't locate remove file: %v", err)
	}

	remF, err := os.OpenFile("/sys/bus/rbd/remove", os.O_WRONLY, 0200)
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
