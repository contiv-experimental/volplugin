package cephdriver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/librbd"
)

const (
	defaultDeviceBase = "/dev/rbd"
	defaultMountBase  = "/mnt/ceph"
)

// Ceph driver object
type CephDriver struct {
	deviceBase string
	mountBase  string
	pool       *librbd.Pool
	rbdConfig  librbd.RBDConfig
	PoolName   string
}

// Volume specification
type CephVolumeSpec struct {
	VolumeName string // Name of the volume
	VolumeSize uint64 // Size in MBs
}

// Create a new Ceph driver
func NewCephDriver(config librbd.RBDConfig, poolName string) (*CephDriver, error) {
	pool, err := librbd.GetPool(config, poolName)
	if err != nil {
		return nil, err
	}

	return &CephDriver{
		deviceBase: defaultDeviceBase,
		mountBase:  defaultMountBase,
		PoolName:   poolName,
		pool:       pool,
		rbdConfig:  config,
	}, nil
}

func (self *CephDriver) MountPath(volumeName string) string {
	return filepath.Join(self.mountBase, self.PoolName, volumeName)
}

func (self *CephDriver) volumeCreate(volumeName string, volumeSize uint64) error {
	return self.pool.CreateImage(volumeName, volumeSize)
}

func (self *CephDriver) mapImage(volumeName string) (string, error) {
	blkdev, err := self.pool.MapDevice(volumeName)
	log.Debugf("mapped volume %q as %q", volumeName, blkdev)

	return blkdev, err
}

func (self *CephDriver) mkfsVolume(devicePath string) error {
	// Create ext4 filesystem on the device. this will take a while
	out, err := exec.Command("mkfs.ext4", "-m0", devicePath).CombinedOutput()

	if err != nil {
		log.Debug(string(out))
		return fmt.Errorf("Error creating ext4 filesystem on %s. Error: %v", devicePath, err)
	}

	return nil
}

func (self *CephDriver) unmapImage(volumeName string) error {
	return self.pool.UnmapDevice(volumeName)
}

func (self *CephDriver) volumeExists(volumeName string) (bool, error) {
	list, err := self.pool.List()
	if err != nil {
		return false, err
	}

	for _, volName := range list {
		if volName == volumeName {
			return true, nil
		}
	}

	return false, nil
}

// Create an RBD image and initialize ext4 filesystem on the image
func (self *CephDriver) CreateVolume(spec CephVolumeSpec) error {
	if ok, err := self.volumeExists(spec.VolumeName); ok && err == nil {
		return nil
	} else if err != nil {
		return err
	}

	if err := self.volumeCreate(spec.VolumeName, spec.VolumeSize); err != nil {
		return err
	}

	blkdev, err := self.mapImage(spec.VolumeName)
	if err != nil {
		return err
	}

	if err := self.mkfsVolume(blkdev); err != nil {
		return err
	}

	if err := self.unmapImage(spec.VolumeName); err != nil {
		return err
	}

	return nil
}

// Map an RBD image and mount it on /mnt/ceph/<datastore>/<volume> directory
// FIXME: Figure out how to use rbd locks
func (self *CephDriver) MountVolume(spec CephVolumeSpec) error {
	// Directory to mount the volume
	dataStoreDir := filepath.Join(self.mountBase, self.PoolName)
	volumeDir := filepath.Join(dataStoreDir, spec.VolumeName)

	devName, err := self.mapImage(spec.VolumeName)
	if err != nil {
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
	// Directory to mount the volume
	dataStoreDir := filepath.Join(self.mountBase, self.PoolName)
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
		return fmt.Errorf("Failed to unmount %q: %v", volumeDir, err)
	}

	// Remove the mounted directory
	if err := os.Remove(volumeDir); err != nil && !os.IsNotExist(err) {
		if err, ok := err.(*os.PathError); ok && err.Err == syscall.EBUSY {
			return nil
		}

		return fmt.Errorf("error removing %q directory: %v", volumeDir, err)
	}

	if err := self.unmapImage(spec.VolumeName); err != os.ErrNotExist {
		return err
	}

	return nil
}

// Delete an RBD volume i.e. rbd image
func (self *CephDriver) DeleteVolume(spec CephVolumeSpec) error {
	return self.pool.RemoveImage(spec.VolumeName)
}
