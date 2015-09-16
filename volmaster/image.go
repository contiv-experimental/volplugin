package main

import (
	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"
)

func createImage(config *config.TenantConfig, pool, name string) error {
	return cephdriver.NewCephDriver().NewVolume(pool, name, config.Size).Create()
}

func removeImage(pool, name string) error {
	return cephdriver.NewCephDriver().NewVolume(pool, name, 0).Remove()
}
