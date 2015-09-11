package main

import (
	"fmt"
	"time"

	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"

	log "github.com/Sirupsen/logrus"
)

func wrapSnapshotAction(config *config.TopLevelConfig, action func(config *config.TopLevelConfig, tenant string, volume *cephdriver.CephVolume)) {
	for tenant, value := range config.Tenants {
		mutex.Lock()
		duration, err := time.ParseDuration(config.Tenants[tenant].Snapshot.Frequency)
		if err != nil {
			panic(fmt.Sprintf("Runtime configuration incorrect; cannot use %q as a snapshot frequency", config.Tenants[tenant].Snapshot.Frequency))
		}

		if value.UseSnapshots && time.Now().Unix()%int64(duration.Seconds()) == 0 {
			for _, volumes := range volumeMap {
				driver := cephdriver.NewCephDriver(config.Tenants[tenant].Pool)
				for volName := range volumes {
					volume := driver.NewVolume(volName, config.Tenants[tenant].Size)
					action(config, tenant, volume)
				}
			}
		}
		mutex.Unlock()
	}
}

func scheduleSnapshotPrune(config *config.TopLevelConfig) {
	for {
		log.Debug("Running snapshot prune supervisor")

		wrapSnapshotAction(config, runSnapshotPrune)

		time.Sleep(1 * time.Second)
	}
}

func runSnapshotPrune(config *config.TopLevelConfig, tenant string, volume *cephdriver.CephVolume) {
	log.Debugf("starting snapshot prune for %q %v", tenant, volume)
	list, err := volume.ListSnapshots()
	if err != nil {
		log.Errorf("Could not list snapshots for tenant %q, volume %v", tenant, volume)
		return
	}

	toDeleteCount := len(list) - int(config.Tenants[tenant].Snapshot.Keep)
	if toDeleteCount < 0 {
		return
	}

	for i := 0; i < toDeleteCount; i++ {
		log.Infof("Removing snapshot %q for tenant %q, volume %v", list[i], tenant, volume)
		if err := volume.RemoveSnapshot(list[i]); err != nil {
			log.Errorf("Removing snapshot %q for tenant %q, volume %v failed: %v", list[i], tenant, volume, err)
		}
	}
}

func runSnapshot(config *config.TopLevelConfig, tenant string, volume *cephdriver.CephVolume) {
	now := time.Now()
	log.Infof("Snapping volume \"%s/%s\" at %v", tenant, volume, now)
	if err := volume.CreateSnapshot(now.String()); err != nil {
		log.Errorf("Cannot snap volume: tenant %q, %q: %v", volume, err)
	}
}

func scheduleSnapshots(config *config.TopLevelConfig) {
	for {
		log.Debug("Running snapshot supervisor")

		wrapSnapshotAction(config, runSnapshot)

		time.Sleep(1 * time.Second)
	}
}
