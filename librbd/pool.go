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
import "C"
import (
	"runtime"
	"strings"
	"unsafe"
)

// Pool is a unit of storage composed of many images.
type Pool struct {
	ioctx     C.rados_ioctx_t
	cluster   C.rados_t
	poolName  string
	rbdConfig RBDConfig
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

// GetImage find and returns an Image struct for an existing image. If the
// image could not be fetched, or does not exist, this function will return
// error.
func (p *Pool) GetImage(imageName string) (*Image, error) {
	// use this so we can exploit wrapOpen() to determine if the image exists
	action := func(image *C.rbd_image_t) error { return nil }

	if err := p.wrapOpen(imageName, action); err != nil {
		return nil, err
	}

	return &Image{imageName: imageName, pool: p}, nil
}
