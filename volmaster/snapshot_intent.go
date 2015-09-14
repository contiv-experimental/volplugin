package main

import (
	"fmt"
	"time"

	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"

	log "github.com/Sirupsen/logrus"
)

func wrapSnapshotAction(config *config.TopLevelConfig, action func(config *config.TopLevelConfig, volName string, volume *config.TenantConfig)) {
	volumes, err := config.ListVolumes()
	if err != nil {
		panic(fmt.Sprintf("Runtime configuration incorrect: %v", err))
	}

	for volName, volume := range volumes {
		duration, err := time.ParseDuration(volume.Snapshot.Frequency)
		if err != nil {
			panic(fmt.Sprintf("Runtime configuration incorrect; cannot use %q as a snapshot frequency", volume.Snapshot.Frequency))
		}

		if volume.UseSnapshots && time.Now().Unix()%int64(duration.Seconds()) == 0 {
			action(config, volName, volume)
		}
	}
}

func scheduleSnapshotPrune(config *config.TopLevelConfig) {
	for {
		log.Debug("Running snapshot prune supervisor")

		wrapSnapshotAction(config, runSnapshotPrune)

		time.Sleep(1 * time.Second)
	}
}

func runSnapshotPrune(config *config.TopLevelConfig, volName string, volume *config.TenantConfig) {
	cephVol := cephdriver.NewCephDriver(volume.Pool).NewVolume(volName, volume.Size)
	log.Debugf("starting snapshot prune for %q", volName)
	list, err := cephVol.ListSnapshots()
	if err != nil {
		log.Errorf("Could not list snapshots for volume %q", volume)
		return
	}

	toDeleteCount := len(list) - int(volume.Snapshot.Keep)
	if toDeleteCount < 0 {
		return
	}

	for i := 0; i < toDeleteCount; i++ {
		log.Infof("Removing snapshot %q for  volume %q", list[i], volume)
		if err := cephVol.RemoveSnapshot(list[i]); err != nil {
			log.Errorf("Removing snapshot %q for volume %q failed: %v", list[i], volume, err)
		}
	}
}

func runSnapshot(config *config.TopLevelConfig, volName string, volume *config.TenantConfig) {
	now := time.Now()
	cephVol := cephdriver.NewCephDriver(volume.Pool).NewVolume(volName, volume.Size)
	log.Infof("Snapping volume %q at %v", volume, now)
	if err := cephVol.CreateSnapshot(now.String()); err != nil {
		log.Errorf("Cannot snap volume: %q: %v", volName, err)
	}
}

func scheduleSnapshots(config *config.TopLevelConfig) {
	for {
		log.Debug("Running snapshot supervisor")

		wrapSnapshotAction(config, runSnapshot)

		time.Sleep(1 * time.Second)
	}
}
