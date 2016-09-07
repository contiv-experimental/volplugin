package ceph

import (
	"fmt"
	"path/filepath"

	"github.com/contiv/volplugin/storage"
)

// MountPath returns the path of a mount for a pool/volume.
func (c *Driver) MountPath(do storage.DriverOptions) (string, error) {
	volName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return "", err
	}

	var poolName string
	if err := do.Volume.Params.Get("pool", &poolName); err != nil {
		return "", err
	}

	return filepath.Join(c.mountpath, poolName, volName), nil
}

// FIXME maybe this belongs in storage/ as it's more general?
func templateFSCmd(fscmd, devicePath string) string {
	for idx := 0; idx < len(fscmd); idx++ {
		if fscmd[idx] == '%' {
			if idx < len(fscmd)-1 && fscmd[idx+1] == '%' {
				idx++
				continue
			}
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
		}
	}

	return fscmd
}
