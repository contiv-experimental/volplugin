package librbd

// #cgo LDFLAGS: -lrbd -lrados
// #include <rados/librados.h>
// #include <rbd/librbd.h>
// #include <stdlib.h>
// rbd_image_t*  make_image() {
//   return malloc(sizeof(rbd_image_t));
// }
import "C"
import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

func (p *Pool) findDevice(imageName string) (string, error) {
	if name, err := p.findDeviceTree(imageName); err == nil {
		if _, err := os.Stat(rbdDev + name); err != nil {
			return "", err
		}

		return rbdDev + name, nil
	}

	return "", os.ErrNotExist
}

func (p *Pool) findDeviceTree(imageName string) (string, error) {
	fi, err := ioutil.ReadDir(rbdDevicePath)
	if err != nil && err != os.ErrNotExist {
		return "", err
	} else if err == os.ErrNotExist {
		return "", fmt.Errorf("Could not locate devices directory")
	}

	for _, f := range fi {
		namePath := filepath.Join(rbdDevicePath, f.Name(), "name")
		content, err := ioutil.ReadFile(namePath)
		if err != nil {
			return "", err
		}

		if strings.TrimSpace(string(content)) == imageName {
			poolPath := filepath.Join(rbdDevicePath, f.Name(), "pool")
			content, err := ioutil.ReadFile(poolPath)
			if err != nil {
				return "", err
			}

			if strings.TrimSpace(string(content)) == p.poolName {
				return f.Name(), err
			}
		}
	}

	return "", os.ErrNotExist
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
