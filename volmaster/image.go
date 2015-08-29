package main

import (
	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/librbd"
)

func createImage(config configTenant, name string, size uint64) error {
	rbdConfig, err := librbd.ReadConfig("/etc/rbdconfig.json")
	if err != nil {
		return err
	}

	driver, err := cephdriver.NewCephDriver(rbdConfig, config.Pool)
	if err != nil {
		return err
	}

	if err := driver.NewVolume(name, config.Size).Create(); err != nil {
		return err
	}

	return nil
}
