package volplugin

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"
	"github.com/contiv/volplugin/api"
)

func (dc *DaemonConfig) mountExists(driver storage.MountDriver, driverOpts storage.DriverOptions) (bool, error) {
	mounts, err := driver.Mounted(dc.Global.Timeout)
	if err != nil {
		return false, err
	}

	mountPath, err := driver.MountPath(driverOpts)
	if err != nil {
		return false, err
	}

	for _, mount := range mounts {
		if mount.Path == mountPath {
			return true, nil
		}
	}

	return false, nil
}

func (dc *DaemonConfig) volumeToDriverOptions(volConfig *config.Volume) (storage.DriverOptions, error) {
	actualSize, err := volConfig.CreateOptions.ActualSize()
	if err != nil {
		return storage.DriverOptions{}, err
	}

	return storage.DriverOptions{
		Volume: storage.Volume{
			Source: volConfig.MountSource,
			Name:   volConfig.String(),
			Size:   actualSize,
			Params: volConfig.DriverOptions,
		},
		FSOptions: storage.FSOptions{
			Type: volConfig.CreateOptions.FileSystem,
		},
		Timeout: dc.Global.Timeout,
	}, nil
}

func (dc *DaemonConfig) structsVolumeName(uc *unmarshalledConfig) (storage.MountDriver, *config.Volume, storage.DriverOptions, error) {
	driverOpts := storage.DriverOptions{}
	volConfig, err := dc.Client.GetVolume(uc.Policy, uc.Name)
	if err != nil {
		return nil, nil, driverOpts, err
	}

	driver, err := backend.NewMountDriver(volConfig.Backends.Mount, dc.Global.MountPath)
	if err != nil {
		return nil, nil, driverOpts, errors.GetDriver.Combine(err)
	}

	driverOpts, err = dc.volumeToDriverOptions(volConfig)
	if err != nil {
		return nil, nil, driverOpts, errors.UnmarshalRequest.Combine(err)
	}

	return driver, volConfig, driverOpts, nil
}

func unmarshalRequest(body io.Reader) (api.VolumeCreateRequest, error) {
	vr := api.VolumeCreateRequest{}

	content, err := ioutil.ReadAll(body)
	if err != nil {
		return vr, err
	}

	err = json.Unmarshal(content, &vr)
	return vr, err
}

func splitPath(name string) (string, string, error) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		return "", "", errored.Errorf("Invalid volume name %q", name)
	}

	return parts[0], parts[1], nil
}
