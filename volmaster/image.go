package main

import "github.com/contiv/volplugin/cephdriver"

func createImage(config configTenant, name string, size uint64) error {
	driver := cephdriver.NewCephDriver(config.Pool)

	if err := driver.NewVolume(name, config.Size).Create(); err != nil {
		return err
	}

	return nil
}
