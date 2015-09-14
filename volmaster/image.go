package main

import (
	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"
)

func createImage(config *config.TenantConfig, name string) error {
	driver := cephdriver.NewCephDriver(config.Pool)

	if err := driver.NewVolume(name, config.Size).Create(); err != nil {
		return err
	}

	return nil
}
