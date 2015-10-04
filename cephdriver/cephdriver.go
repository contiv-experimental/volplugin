package cephdriver

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
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
}

// NewCephDriver creates a new Ceph driver with default paths for mounting and
// device mapping.
func NewCephDriver() *CephDriver {
	return &CephDriver{
		deviceBase: defaultDeviceBase,
		mountBase:  defaultMountBase,
	}
}

// PoolExists determines if a pool exists.
func (cd *CephDriver) PoolExists(poolName string) (bool, error) {
	cmd := exec.Command("ceph", "osd", "pool", "ls")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == poolName {
			return true, nil
		}
	}

	return false, nil
}

// MountPath joins the necessary parts to find the mount point for the volume
// name.
func (cd *CephDriver) MountPath(poolName, volumeName string) string {
	return filepath.Join(cd.mountBase, poolName, volumeName)
}

// NewVolume returns a *CephVolume ready for use with volume operations.
func (cd *CephDriver) NewVolume(poolName, volumeName string, size uint64) *CephVolume {
	return &CephVolume{
		VolumeName: volumeName,
		PoolName:   poolName,
		VolumeSize: size,
		driver:     cd,
	}
}

func templateFsCmd(fscmd, devicePath string) string {
	insidePercent := false

	for idx := 0; idx < len(fscmd); idx++ {
		if fscmd[idx] == '%' {
			if insidePercent {
				insidePercent = false
				continue
			}

			insidePercent = true
			var lhs, rhs string

			switch {
			case idx == 0:
				lhs = ""
				rhs = fscmd[1:]
			case idx == len(fscmd)-1:
				lhs = fscmd[:idx]
				rhs = ""
			default:
				lhs = fscmd[:idx]
				rhs = fscmd[idx+1:]
			}

			fscmd = fmt.Sprintf("%s%s%s", lhs, devicePath, rhs)
		} else {
			insidePercent = false
		}
	}

	return fscmd
}
