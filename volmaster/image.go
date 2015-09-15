package main

import (
	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"
)

func createImage(config *config.TenantConfig, pool, name string) error {
	driver := cephdriver.NewCephDriver()

	if err := driver.NewVolume(pool, name, config.Size).Create(); err != nil {
		return err
	}

	return nil
}
