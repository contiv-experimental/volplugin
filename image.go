package librbd

// #cgo LDFLAGS: -lrbd -lrados
// #include <rados/librados.h>
// #include <rbd/librbd.h>
// #include <stdlib.h>
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

// Image is used to manipulate the properties of a specific image.
type Image struct {
	imageName string
	pool      *Pool
}

// ListSnapshots yields a list of the snapshots for the given interface. max
// will yield a maximum of N items.
func (img *Image) ListSnapshots(max int) ([]string, error) {
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

	if err := img.pool.wrapOpen(img.imageName, action); err != nil {
		return nil, err
	}

	return list, nil
}

// CreateSnapshot creates a named snapshot for the image provided.
func (img *Image) CreateSnapshot(snapshotName string) error {
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

	if err := img.pool.wrapOpen(img.imageName, action); err != nil {
		return err
	}

	return nil
}

// RemoveSnapshot deletes a named snapshot for the image provided.
func (img *Image) RemoveSnapshot(snapshotName string) error {
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

	if err := img.pool.wrapOpen(img.imageName, action); err != nil {
		return err
	}

	return nil
}

// MapDevice maps an image to a device on the host. Returns the device path and
// any errors. On error, the device path will be blank.
func (img *Image) MapDevice() (string, error) {
	if str, err := img.pool.findDevice(img.imageName); err == nil {
		return str, nil
	}

	addF, err := os.OpenFile(filepath.Join(rbdBusPath, "add"), os.O_WRONLY, 0200)
	if err != nil {
		return "", err
	}

	defer addF.Close()

	output := fmt.Sprintf("%s name=%s,secret=%s %s %s", img.pool.rbdConfig.MonitorIP, img.pool.rbdConfig.UserName, img.pool.rbdConfig.Secret, img.pool.poolName, img.imageName)

	if _, err := addF.Write([]byte(output)); err != nil {
		if err != nil {
			return "", err
		}
	}

	return img.pool.findDevice(img.imageName)
}

// UnmapDevice removes a RBD device. Given an image name, it will locate the
// device and unmap it. Returns an error on any failure.
func (img *Image) UnmapDevice() error {
	devName, err := img.pool.findDevice(img.imageName)
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
