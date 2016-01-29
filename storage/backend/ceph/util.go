package ceph

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/contiv/volplugin/executor"
)

func (c *Driver) poolExists(poolName string) (bool, error) {
	er, err := executor.New(exec.Command("ceph", "osd", "pool", "ls")).Run()
	if err != nil {
		return false, fmt.Errorf("Problem listing pools: %v", err)
	}

	if er.ExitStatus != 0 {
		return false, fmt.Errorf("Problem listing pools: %v", er)
	}

	lines := strings.Split(er.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == poolName {
			return true, nil
		}
	}

	return false, nil
}

// MountPath returns the path of a mount for a pool/volume.
func MountPath(poolName, volumeName string) string {
	return filepath.Join(mountBase, poolName, volumeName)
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
