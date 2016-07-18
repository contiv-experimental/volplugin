package volplugin

import (
	"github.com/contiv/volplugin/api/docker"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"
)

func (dc *DaemonConfig) structsVolumeName(uc *docker.Request) (storage.MountDriver, *config.Volume, storage.DriverOptions, error) {
	driverOpts := storage.DriverOptions{}
	volConfig, err := dc.Client.GetVolume(uc.Policy, uc.Name)
	if err != nil {
		return nil, nil, driverOpts, err
	}

	driver, err := backend.NewMountDriver(volConfig.Backends.Mount, dc.Global.MountPath)
	if err != nil {
		return nil, nil, driverOpts, errors.GetDriver.Combine(err)
	}

	driverOpts, err = volConfig.ToDriverOptions(dc.Global.Timeout)
	if err != nil {
		return nil, nil, driverOpts, errors.UnmarshalRequest.Combine(err)
	}

	return driver, volConfig, driverOpts, nil
}
