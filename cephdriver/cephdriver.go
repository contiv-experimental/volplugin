package cephdriver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
)

const (
	defaultDeviceBase = "/dev/rbd"
	defaultMountBase  = "/mnt/ceph"
)

// Ceph driver object
type CephDriver struct {
	deviceBase string
	mountBase  string
}

// Volume specification
type CephVolumeSpec struct {
	VolumeName string // Name of the volume
	VolumeSize uint   // Size in MBs
	PoolName   string // Ceph Pool this volume belongs to default:rbd
}

// Create a new Ceph driver
func NewCephDriver() *CephDriver {
	return &CephDriver{
		deviceBase: defaultDeviceBase,
		mountBase:  defaultMountBase,
	}
}

func (cvs *CephVolumeSpec) Path() string {
	return filepath.Join(cvs.PoolName, cvs.VolumeName)
}

func (self *CephDriver) DevicePath(spec CephVolumeSpec) string {
	return filepath.Join(self.deviceBase, spec.Path())
}

func (self *CephDriver) MountPath(spec CephVolumeSpec) string {
	return filepath.Join(self.mountBase, spec.Path())
}

func (self *CephDriver) volumeCreate(spec CephVolumeSpec) error {
	// Create an image
	out, err := exec.Command("rbd", "create", spec.Path(), "--size",
		strconv.Itoa(int(spec.VolumeSize))).CombinedOutput()

	log.Debug(string(out))

	if err != nil {
		return fmt.Errorf("Error creating Ceph RBD image(name: %s, size: %d). Err: %v\n",
			spec.Path(), spec.VolumeSize, err)
	}

	return nil
}

func (self *CephDriver) mapImage(spec CephVolumeSpec) error {
	// Temporarily map the image to create a filesystem
	out, err := exec.Command("rbd", "map", spec.Path()).CombinedOutput()

	log.Debug(string(out))

	if err != nil {
		return fmt.Errorf("Error mapping the image %s. Error: %v", spec.Path(), err)
	}

	return nil
}

func (self *CephDriver) mkfsVolume(spec CephVolumeSpec) error {
	// Create ext4 filesystem on the device. this will take a while
	out, err := exec.Command("mkfs.ext4", "-m0", self.DevicePath(spec)).CombinedOutput()

	log.Debug(string(out))

	if err != nil {
		return fmt.Errorf("Error creating ext4 filesystem on %s. Error: %v", self.DevicePath(spec), err)
	}

	return nil
}

func (self *CephDriver) unmapImage(spec CephVolumeSpec) error {
	// finally, Unmap the rbd image
	out, err := exec.Command("rbd", "unmap", self.DevicePath(spec)).CombinedOutput()

	log.Debug(string(out))

	if err != nil {
		return fmt.Errorf("Error unmapping the device %s. Error: %v", self.DevicePath(spec), err)
	}

	return nil
}

func (self *CephDriver) volumeExists(spec CephVolumeSpec) (bool, error) {
	out, err := exec.Command("rbd", "ls", spec.PoolName).CombinedOutput()
	if err != nil {
		return false, err
	}

	strs := strings.Split(string(out), "\n")
	for _, str := range strs {
		if str == spec.VolumeName {
			return true, nil
		}
	}

	return false, nil
}

// Create an RBD image and initialize ext4 filesystem on the image
func (self *CephDriver) CreateVolume(spec CephVolumeSpec) error {
	if ok, err := self.volumeExists(spec); ok && err == nil {
		return nil
	} else if err != nil {
		return err
	}

	if err := self.volumeCreate(spec); err != nil {
		return err
	}

	if err := self.mapImage(spec); err != nil {
		return err
	}

	if err := self.mkfsVolume(spec); err != nil {
		return err
	}

	if err := self.unmapImage(spec); err != nil {
		return err
	}

	return nil
}

// Map an RBD image and mount it on /mnt/ceph/<datastore>/<volume> directory
// FIXME: Figure out how to use rbd locks
func (self *CephDriver) MountVolume(spec CephVolumeSpec) error {
	// formatted image name
	devName := self.DevicePath(spec)

	// Directory to mount the volume
	dataStoreDir := filepath.Join(self.mountBase, spec.PoolName)
	volumeDir := filepath.Join(dataStoreDir, spec.VolumeName)

	if err := self.mapImage(spec); err != nil {
		return err
	}

	// Create directory to mount
	if err := os.Mkdir(self.mountBase, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating %q directory: %v", self.mountBase, err)
	}

	if err := os.Mkdir(dataStoreDir, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating %q directory: %v", dataStoreDir)
	}

	if err := os.Mkdir(volumeDir, 0777); err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating %q directory: %v", volumeDir)
	}

	// Mount the RBD
	if err := syscall.Mount(devName, volumeDir, "ext4", 0, ""); err != nil && err != syscall.EBUSY {
		return fmt.Errorf("Failed to mount RBD dev %q: %v", devName, err.Error())
	}

	return nil
}

// Unount a Ceph volume, remove the mount directory and unmap the RBD device
func (self *CephDriver) UnmountVolume(spec CephVolumeSpec) error {
	// formatted image name
	devName := self.DevicePath(spec)

	// Directory to mount the volume
	dataStoreDir := filepath.Join(self.mountBase, spec.PoolName)
	volumeDir := filepath.Join(dataStoreDir, spec.VolumeName)

	// Unmount the RBD
	//
	// MNT_DETACH will make this mountpoint unavailable to new open file requests (at
	// least until it is remounted) but persist for existing open requests. This
	// seems to work well with containers.
	//
	// The checks for ENOENT and EBUSY below are safeguards to prevent error
	// modes where multiple containers will be affecting a single volume.
	if err := syscall.Unmount(volumeDir, syscall.MNT_DETACH); err != nil && err != syscall.ENOENT {
		return fmt.Errorf("Failed to unmount %q: %v", devName, err)
	}

	// Remove the mounted directory
	if err := os.Remove(volumeDir); err != nil && !os.IsNotExist(err) {
		if err, ok := err.(*os.PathError); ok && err.Err == syscall.EBUSY {
			return nil
		}

		return fmt.Errorf("error removing %q directory: %v", volumeDir, err)
	}

	if err := self.unmapImage(spec); err != nil {
		return err
	}

	return nil
}

// Delete an RBD volume i.e. rbd image
func (self *CephDriver) DeleteVolume(spec CephVolumeSpec) error {
	out, err := exec.Command("rbd", "rm", spec.Path()).CombinedOutput()

	log.Debug(string(out))

	if err != nil {
		return fmt.Errorf("Error deleting Ceph RBD image(name: %s). Err: %v", spec.Path(), err)
	}

	return nil
}
