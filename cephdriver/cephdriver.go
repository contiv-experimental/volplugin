package cephdriver

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/contiv/volplugin/librbd"
)

const (
	defaultDeviceBase = "/dev/rbd"
	defaultMountBase  = "/mnt/ceph"
)

// CephDriver is the principal struct in this package which corresponds to a
// ceph pool, and its parameters.
type CephDriver struct {
	deviceBase string
	mountBase  string
	pool       *librbd.Pool
	rbdConfig  librbd.RBDConfig
	PoolName   string // Name of Pool, populated by NewCephDriver()
}

// CephVolumeSpec is a struct that communicates volume name and size.
type CephVolumeSpec struct {
	VolumeName string // Name of the volume
	VolumeSize uint64 // Size in MBs
}

// NewCephDriver creates a new Ceph driver
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

func (cvs CephVolumeSpec) String() string {
	return fmt.Sprintf("[name: %s size: %d]", cvs.VolumeName, cvs.VolumeSize)
}

// MountPath joins the necessary parts to find the mount point for the volume
// name.
func (cd *CephDriver) MountPath(volumeName string) string {
	return filepath.Join(cd.mountBase, cd.PoolName, volumeName)
}

// CreateVolume creates an RBD image and initialize ext4 filesystem on the image
func (cd *CephDriver) CreateVolume(spec CephVolumeSpec) error {
	if ok, err := cd.volumeExists(spec.VolumeName); ok && err == nil {
		return nil
	} else if err != nil {
		return err
	}

	if err := cd.volumeCreate(spec.VolumeName, spec.VolumeSize); err != nil {
		return err
	}

	blkdev, err := cd.mapImage(spec.VolumeName)
	if err != nil {
		return err
	}

	if err := cd.mkfsVolume(blkdev); err != nil {
		return err
	}

	if err := cd.unmapImage(spec.VolumeName); err != nil {
		return err
	}

	return nil
}

// MountVolume maps an RBD image and mount it on /mnt/ceph/<datastore>/<volume> directory
// FIXME: Figure out how to use rbd locks
func (cd *CephDriver) MountVolume(spec CephVolumeSpec) error {
	// Directory to mount the volume
	dataStoreDir := filepath.Join(cd.mountBase, cd.PoolName)
	volumeDir := filepath.Join(dataStoreDir, spec.VolumeName)

	devName, err := cd.mapImage(spec.VolumeName)
	if err != nil {
		return err
	}

	// Create directory to mount
	if err := os.Mkdir(cd.mountBase, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating %q directory: %v", cd.mountBase, err)
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

// UnmountVolume unmounts a Ceph volume, remove the mount directory and unmap
// the RBD device
func (cd *CephDriver) UnmountVolume(spec CephVolumeSpec) error {
	// formatted image name
	// Directory to mount the volume
	dataStoreDir := filepath.Join(cd.mountBase, cd.PoolName)
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

	if err := cd.unmapImage(spec.VolumeName); err != os.ErrNotExist {
		return err
	}

	return nil
}

// DeleteVolume deletes an RBD volume i.e. rbd image
func (cd *CephDriver) DeleteVolume(spec CephVolumeSpec) error {
	return cd.pool.RemoveImage(spec.VolumeName)
}
