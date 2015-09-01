package cephdriver

import (
	"path/filepath"
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
	PoolName   string // Name of Pool, populated by NewCephDriver()
}

// NewCephDriver creates a new Ceph driver
func NewCephDriver(poolName string) (*CephDriver, error) {
	return &CephDriver{
		deviceBase: defaultDeviceBase,
		mountBase:  defaultMountBase,
		PoolName:   poolName,
	}, nil
}

// MountPath joins the necessary parts to find the mount point for the volume
// name.
func (cd *CephDriver) MountPath(volumeName string) string {
	return filepath.Join(cd.mountBase, cd.PoolName, volumeName)
}

// NewVolume returns a *CephVolume ready for use with volume operations.
func (cd *CephDriver) NewVolume(volumeName string, size uint64) *CephVolume {
	return &CephVolume{VolumeName: volumeName, VolumeSize: size, driver: cd}
}
