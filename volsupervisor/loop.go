package volsupervisor

import (
	"strings"
	"sync"
	"time"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"

	log "github.com/Sirupsen/logrus"
)

var (
	volumes     = map[string]*config.VolumeConfig{}
	volumeMutex = &sync.Mutex{}
)

func (dc *DaemonConfig) pruneSnapshots(volume string, val *config.VolumeConfig) {
	log.Infof("starting snapshot prune for %q", val.VolumeName)

	uc := &config.UseSnapshot{
		Volume: val,
		Reason: lock.ReasonSnapshotPrune,
	}

	err := lock.NewDriver(dc.Config).ExecuteWithUseLock(uc, func(ld *lock.Driver, uc config.UseLocker) error {
		driver, err := backend.NewDriver(val.Options.Backend, dc.Global.MountPath)
		if err != nil {
			log.Errorf("failed to get driver: %v", err)
			return err
		}

		driverOpts := storage.DriverOptions{
			Volume: storage.Volume{
				Name: strings.Join([]string{val.PolicyName, val.VolumeName}, "."),
				Params: storage.Params{
					"pool": val.Options.Pool,
				},
			},
			Timeout: dc.Global.Timeout,
		}

		list, err := driver.ListSnapshots(driverOpts)
		if err != nil {
			log.Errorf("Could not list snapshots for volume %q: %v", val.VolumeName, err)
			return err
		}

		log.Debugf("Volume %q: keeping %d snapshots", val, val.Options.Snapshot.Keep)

		toDeleteCount := len(list) - int(val.Options.Snapshot.Keep)
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

func (dc *DaemonConfig) createSnapshot(volume string, val *config.VolumeConfig) {
	log.Infof("Snapshotting %q.", volume)

	uc := &config.UseSnapshot{
		Volume: val,
		Reason: lock.ReasonSnapshot,
	}

	err := lock.NewDriver(dc.Config).ExecuteWithUseLock(uc, func(ld *lock.Driver, uc config.UseLocker) error {
		driver, err := backend.NewDriver(val.Options.Backend, dc.Global.MountPath)
		if err != nil {
			log.Errorf("Error establishing driver backend %q; cannot snapshot", val.Options.Backend)
			return err
		}

		driverOpts := storage.DriverOptions{
			Volume: storage.Volume{
				Name: strings.Join([]string{val.PolicyName, val.VolumeName}, "."),
				Params: storage.Params{
					"pool": val.Options.Pool,
				},
			},
			Timeout: dc.Global.Timeout,
		}

		if err := driver.CreateSnapshot(time.Now().String(), driverOpts); err != nil {
			log.Errorf("Error creating snapshot for volume %q: %v", volume, err)
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
		volumeCopy := map[string]*config.VolumeConfig{}

		volumeMutex.Lock()
		for volume, val := range volumes {
			volumeCopy[volume] = val
		}
		volumeMutex.Unlock()

		for volume, val := range volumeCopy {
			if val.Options.UseSnapshots {
				freq, err := time.ParseDuration(val.Options.Snapshot.Frequency)
				if err != nil {
					log.Errorf("Volume %q has an invalid frequency. Skipping snapshot.", volume)
				}

				if time.Now().Unix()%int64(freq.Seconds()) == 0 {
					go func(volume string, val *config.VolumeConfig) {
						dc.createSnapshot(volume, val)
						dc.pruneSnapshots(volume, val)
					}(volume, val)
				}
			}
		}
	}
}
