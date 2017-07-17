package nfs

import (
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/mountscan"
)

// Mounted shows any volumes that belong to volplugin on the host, in
// their native representation. They yield a *Mount.
func (d *Driver) Mounted(timeout time.Duration) ([]*storage.Mount, error) {
	mounts := []*storage.Mount{}
	hostMounts, err := mountscan.GetMounts(&mountscan.GetMountsRequest{DriverName: "nfs", FsType: "nfs4"})
	if err != nil {
		if newerr, ok := err.(*errored.Error); ok && newerr.Contains(errors.ErrDevNotFound) {
			return mounts, nil
		}
		return nil, err
	}

	for _, hostMount := range hostMounts {
		rel, err := filepath.Rel(d.mountpath, hostMount.MountPoint)
		if err != nil {
			logrus.Errorf("Invalid volume calucated from mountpoint %q with mountpath %q", hostMount.MountPoint, d.mountpath)
			continue
		}
		mounts = append(mounts, &storage.Mount{
			DevMajor: hostMount.DeviceNumber.Major,
			DevMinor: hostMount.DeviceNumber.Minor,
			Path:     hostMount.MountPoint,
			Volume: storage.Volume{
				Name: rel,
				Params: storage.DriverParams{
					"mount": hostMount.MountSource,
				},
			},
		})
	}
	return mounts, nil
}
