package main

import (
	log "github.com/Sirupsen/logrus"

	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"
)

func createImage(config *config.VolumeConfig, pool, name string) error {
	log.Printf("Opts: %d", config.Options.Size)
	return cephdriver.NewCephDriver().NewVolume(pool, name, config.Options.Size).Create()
}

func removeImage(pool, name string) error {
	return cephdriver.NewCephDriver().NewVolume(pool, name, 0).Remove()
}
