package main

import (
	"fmt"
	"time"

	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/librbd"

	log "github.com/Sirupsen/logrus"
)

func scheduleSnapshots(config config) {
	for {
		log.Debug("Running snapshot supervisor")

		for tenant, value := range config {
			mutex.Lock()
			duration, err := time.ParseDuration(config[tenant].Snapshot.Frequency)
			if err != nil {
				panic(fmt.Sprintf("Runtime configuration incorrect; cannot use %q as a snapshot frequency", config[tenant].Snapshot.Frequency))
			}

			if value.UseSnapshots && time.Now().Unix()%int64(duration.Seconds()) == 0 {
				for _, volumes := range volumeMap {
					rbdConfig, err := librbd.ReadConfig("/etc/rbdconfig.json")
					if err != nil {
						log.Errorf("Cannot read RBD configuration: %v", err)
						break
					}
					driver, err := cephdriver.NewCephDriver(rbdConfig, config[tenant].Pool)
					if err != nil {
						log.Errorf("Cannot snap volumes for tenant %q: %v", tenant, err)
						break
					}
					for volume := range volumes {
						now := time.Now()
						log.Infof("Snapping volume \"%s/%s\" at %v", tenant, volume, now)
						if err := driver.NewVolume(volume, config[tenant].Size).CreateSnapshot(now.String()); err != nil {
							log.Errorf("Cannot snap volume %q: %v", volume, err)
						}
					}
				}
			}
			mutex.Unlock()
		}

		time.Sleep(1 * time.Second)
	}
}
