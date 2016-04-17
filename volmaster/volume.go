package volmaster

import (
	"errors"
	"time"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"

	log "github.com/Sirupsen/logrus"
)

const defaultFsCmd = "mkfs.ext4 -m0 %"

var errNoActionTaken = errors.New("No action taken")

func (dc *DaemonConfig) createVolume(policy *config.Policy, config *config.Volume, timeout time.Duration) (storage.DriverOptions, error) {
	var (
		fscmd string
		ok    bool
	)

	if config.Backends.CRUD == "" {
		log.Debugf("Not creating volume %q, backend is unspecified", config)
		return storage.DriverOptions{}, errNoActionTaken
	}

	if policy.FileSystems == nil {
		fscmd = defaultFsCmd
	} else {
		fscmd, ok = policy.FileSystems[config.CreateOptions.FileSystem]
		if !ok {
			return storage.DriverOptions{}, errored.Errorf("Invalid filesystem %q", config.CreateOptions.FileSystem)
		}
	}

	actualSize, err := config.CreateOptions.ActualSize()
	if err != nil {
		return storage.DriverOptions{}, err
	}

	driver, err := backend.NewCRUDDriver(config.Backends.CRUD)
	if err != nil {
		return storage.DriverOptions{}, err
	}

	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name:   config.String(),
			Size:   actualSize,
			Params: config.DriverOptions,
		},
		FSOptions: storage.FSOptions{
			Type:          config.CreateOptions.FileSystem,
			CreateCommand: fscmd,
		},
		Timeout: timeout,
	}

	log.Infof("Creating volume %v with size %d", config, actualSize)
	return driverOpts, driver.Create(driverOpts)
}

func (dc *DaemonConfig) formatVolume(config *config.Volume, do storage.DriverOptions) error {
	actualSize, err := config.CreateOptions.ActualSize()
	if err != nil {
		return err
	}

	if config.Backends.CRUD == "" {
		log.Debugf("Not formatting volume %q, backend is unspecified", config)
		return errNoActionTaken
	}

	driver, err := backend.NewCRUDDriver(config.Backends.CRUD)
	if err != nil {
		return err
	}

	log.Infof("Formatting volume %v (filesystem %q) with size %d", config, config.CreateOptions.FileSystem, actualSize)
	return driver.Format(do)
}

func (dc *DaemonConfig) existsVolume(config *config.Volume) (bool, error) {
	if config.Backends.CRUD == "" {
		log.Debugf("volume %q, backend is unspecified", config)
		return true, errNoActionTaken
	}

	driver, err := backend.NewCRUDDriver(config.Backends.CRUD)
	if err != nil {
		return false, err
	}

	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name:   config.String(),
			Params: config.DriverOptions,
		},
		Timeout: dc.Global.Timeout,
	}

	return driver.Exists(driverOpts)
}

func (dc *DaemonConfig) removeVolume(config *config.Volume, timeout time.Duration) error {
	if config.Backends.CRUD == "" {
		log.Debugf("Not removing volume %q, backend is unspecified", config)
		return errNoActionTaken
	}

	driver, err := backend.NewCRUDDriver(config.Backends.CRUD)
	if err != nil {
		return err
	}

	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name:   config.String(),
			Params: config.DriverOptions,
		},
		Timeout: timeout,
	}

	log.Infof("Destroying volume %v", config)

	return driver.Destroy(driverOpts)
}
