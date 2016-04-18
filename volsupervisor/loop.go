package volsupervisor

import (
	"sync"
	"time"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"

	log "github.com/Sirupsen/logrus"
)

var (
	volumes     = map[string]*config.Volume{}
	volumeMutex = &sync.Mutex{}
)

func (dc *DaemonConfig) pruneSnapshots(val *config.Volume) {
	log.Infof("starting snapshot prune for %q", val.VolumeName)

	uc := &config.UseSnapshot{
		Volume: val.String(),
		Reason: lock.ReasonSnapshotPrune,
	}

	err := lock.NewDriver(dc.Config).ExecuteWithUseLock(uc, func(ld *lock.Driver, uc config.UseLocker) error {
		driver, err := backend.NewSnapshotDriver(val.Backend)
		if err != nil {
			log.Errorf("failed to get driver: %v", err)
			return err
		}

		driverOpts := storage.DriverOptions{
			Volume: storage.Volume{
				Name: val.String(),
				Params: storage.Params{
					"pool": val.DriverOptions["pool"],
				},
			},
			Timeout: dc.Global.Timeout,
		}

		list, err := driver.ListSnapshots(driverOpts)
		if err != nil {
			log.Errorf("Could not list snapshots for volume %q: %v", val.VolumeName, err)
			return err
		}

		log.Debugf("Volume %q: keeping %d snapshots", val, val.RuntimeOptions.Snapshot.Keep)

		toDeleteCount := len(list) - int(val.RuntimeOptions.Snapshot.Keep)
		if toDeleteCount < 0 {
			return nil
		}

		for i := 0; i < toDeleteCount; i++ {
			log.Infof("Removing snapshot %q for volume %q", list[i], val.VolumeName)
			if err := driver.RemoveSnapshot(list[i], driverOpts); err != nil {
				log.Errorf("Removing snapshot %q for volume %q failed: %v", list[i], val.VolumeName, err)
			}
		}

		return nil
	})

	if err != nil {
		log.Errorf("Error removing snapshot for volume %q: %v", val, err)
	}
}

func (dc *DaemonConfig) createSnapshot(val *config.Volume) {
	log.Infof("Snapshotting %q.", val)

	uc := &config.UseSnapshot{
		Volume: val.String(),
		Reason: lock.ReasonSnapshot,
	}

	err := lock.NewDriver(dc.Config).ExecuteWithUseLock(uc, func(ld *lock.Driver, uc config.UseLocker) error {
		driver, err := backend.NewSnapshotDriver(val.Backend)
		if err != nil {
			log.Errorf("Error establishing driver backend %q; cannot snapshot", val.Backend)
			return err
		}

		driverOpts := storage.DriverOptions{
			Volume: storage.Volume{
				Name: val.String(),
				Params: storage.Params{
					"pool": val.DriverOptions["pool"],
				},
			},
			Timeout: dc.Global.Timeout,
		}

		if err := driver.CreateSnapshot(time.Now().String(), driverOpts); err != nil {
			log.Errorf("Error creating snapshot for volume %q: %v", val, err)
			return err
		}

		return nil
	})

	if err != nil {
		log.Errorf("Error creating snapshot for volume %q: %v", val, err)
	}
}

func (dc *DaemonConfig) loop() {
	for {
		time.Sleep(1 * time.Second)

		// XXX this copy is so we can free the mutex quickly for more additions
		volumeCopy := map[string]*config.Volume{}

		volumeMutex.Lock()
		for volume, val := range volumes {
			volumeCopy[volume] = val
		}
		volumeMutex.Unlock()

		for volume, val := range volumeCopy {
			if val.RuntimeOptions.UseSnapshots {
				freq, err := time.ParseDuration(val.RuntimeOptions.Snapshot.Frequency)
				if err != nil {
					log.Errorf("Volume %q has an invalid frequency. Skipping snapshot.", volume)
				}

				if time.Now().Unix()%int64(freq.Seconds()) == 0 {
					go func(val *config.Volume) {
						dc.createSnapshot(val)
						dc.pruneSnapshots(val)
					}(val)
				}
			}
		}
	}
}
