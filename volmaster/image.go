package main

import (
	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"
)

func createImage(config *config.VolumeConfig) error {
	return cephdriver.NewCephDriver().NewVolume(config.Options.Pool, config.VolumeName, config.Options.Size).Create()
}

func removeImage(config *config.VolumeConfig) error {
	return cephdriver.NewCephDriver().NewVolume(config.Options.Pool, config.VolumeName, 0).Remove()
}
