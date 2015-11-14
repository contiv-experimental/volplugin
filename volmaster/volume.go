package volmaster

import (
	"fmt"
	"strings"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend/ceph"
)

const defaultFsCmd = "mkfs.ext4 -m0 %"

func joinVolumeName(config *config.VolumeConfig) string {
	return strings.Join([]string{config.TenantName, config.VolumeName}, ".")
}

func createVolume(tenant *config.TenantConfig, config *config.VolumeConfig) error {
	var (
		fscmd string
		ok    bool
	)

	if tenant.FileSystems == nil {
		fscmd = defaultFsCmd
	} else {
		fscmd, ok = tenant.FileSystems[config.Options.FileSystem]
		if !ok {
			return fmt.Errorf("Invalid filesystem %q", config.Options.FileSystem)
		}
	}

	driver := ceph.NewDriver()
	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name: joinVolumeName(config),
			Size: config.Options.Size,
			Params: storage.Params{
				"pool": config.Options.Pool,
			},
		},
		FSOptions: storage.FSOptions{
			Type:          config.Options.FileSystem,
			CreateCommand: fscmd,
		},
	}

	if err := driver.Create(driverOpts); err != nil {
		return err
	}

	return driver.Format(driverOpts)
}

func removeVolume(config *config.VolumeConfig) error {
	driver := ceph.NewDriver()
	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name: joinVolumeName(config),
			Params: storage.Params{
				"pool": config.Options.Pool,
			},
		},
	}

	return driver.Destroy(driverOpts)
}
