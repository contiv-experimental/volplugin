package nfs

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/storage"
)

const (
	mountInfoFile     = "/proc/self/mountinfo"
	nfsMajorMountType = "0"
)

// Mounted shows any volumes that belong to volplugin on the host, in
// their native representation. They yield a *Mount.
func (d *Driver) Mounted(timeout time.Duration) ([]*storage.Mount, error) {
	mounts := []*storage.Mount{}

	content, err := ioutil.ReadFile(mountInfoFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		parts := strings.Split(line, " ")
		devParts := strings.Split(parts[2], ":")
		if len(devParts) != 2 {
			return nil, errored.Errorf("Could not parse %q properly.", mountInfoFile)
		}

		if devParts[0] == nfsMajorMountType && parts[8] == "nfs4" {
			devMajor, err := strconv.ParseUint(devParts[0], 10, 64)
			if err != nil {
				return nil, errored.Errorf("Invalid device major ID %s reading mount information", devParts[0]).Combine(err)
			}

			devMinor, err := strconv.ParseUint(devParts[1], 10, 64)
			if err != nil {
				return nil, errored.Errorf("Invalid device minor ID %s reading mount information", devParts[1]).Combine(err)
			}

			rel, err := filepath.Rel(d.mountpath, parts[4])
			if err != nil {
				return nil, errored.Errorf("Invalid volume calucated from mountpoint %q with mountpath %q", parts[4], d.mountpath)
			}

			mounts = append(mounts, &storage.Mount{
				DevMajor: uint(devMajor),
				DevMinor: uint(devMinor),
				Path:     parts[4],
				Volume: storage.Volume{
					Name: rel,
					Params: map[string]string{
						"mount": parts[9],
					},
				},
			})
		}
	}

	return mounts, nil
}
