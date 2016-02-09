package volmaster

import (
	"fmt"
	"strings"
	"time"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend/ceph"

	log "github.com/Sirupsen/logrus"
)

const defaultFsCmd = "mkfs.ext4 -m0 %"

func joinVolumeName(config *config.VolumeConfig) string {
	return strings.Join([]string{config.TenantName, config.VolumeName}, ".")
}

func createVolume(tenant *config.TenantConfig, config *config.VolumeConfig, timeout time.Duration) (storage.DriverOptions, error) {
	var (
		fscmd string
		ok    bool
	)

	if tenant.FileSystems == nil {
		fscmd = defaultFsCmd
	} else {
		fscmd, ok = tenant.FileSystems[config.Options.FileSystem]
		if !ok {
			return storage.DriverOptions{}, fmt.Errorf("Invalid filesystem %q", config.Options.FileSystem)
		}
	}

	actualSize, err := config.Options.ActualSize()
	if err != nil {
		return storage.DriverOptions{}, err
	}

	driver := ceph.NewDriver()
	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name: joinVolumeName(config),
			Size: actualSize,
			Params: storage.Params{
				"pool": config.Options.Pool,
			},
		},
		FSOptions: storage.FSOptions{
			Type:          config.Options.FileSystem,
			CreateCommand: fscmd,
		},
		Timeout: timeout,
	}

	log.Infof("Creating volume %q (pool %q) with size %d", joinVolumeName(config), config.Options.Pool, actualSize)
	return driverOpts, driver.Create(driverOpts)
}

func formatVolume(config *config.VolumeConfig, do storage.DriverOptions) error {
	actualSize, err := config.Options.ActualSize()
	if err != nil {
		return err
	}

	log.Infof("Formatting volume %q (pool %q, filesystem %q) with size %d", joinVolumeName(config), config.Options.Pool, config.Options.FileSystem, actualSize)
	return ceph.NewDriver().Format(do)
}

func removeVolume(config *config.VolumeConfig, timeout time.Duration) error {
	driver := ceph.NewDriver()
	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name: joinVolumeName(config),
			Params: storage.Params{
				"pool": config.Options.Pool,
			},
		},
		Timeout: timeout,
	}

	log.Infof("Destroying volume %q (pool %q)", joinVolumeName(config), config.Options.Pool)

	return driver.Destroy(driverOpts)
}
