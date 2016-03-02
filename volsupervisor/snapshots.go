package volsupervisor

import (
	"strings"
	"time"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"

	log "github.com/Sirupsen/logrus"
)

func (dc *DaemonConfig) wrapSnapshotAction(action func(pool string, volume *config.VolumeConfig)) func(*volumeDispatch) {
	return func(v *volumeDispatch) {
		for _, volume := range v.volumes {
			duration, err := time.ParseDuration(volume.Options.Snapshot.Frequency)
			if err != nil {
				log.Errorf("Runtime configuration incorrect; cannot use %q as a snapshot frequency", volume.Options.Snapshot.Frequency)
				return
			}

			if volume.Options.UseSnapshots && time.Now().Unix()%int64(duration.Seconds()) == 0 {
				action(volume.Options.Pool, volume)
			}
		}
	}
}

func (dc *DaemonConfig) scheduleSnapshotPrune() {
	for {
		log.Debug("Running snapshot prune supervisor")

		dc.iterateVolumes(dc.wrapSnapshotAction(dc.runSnapshotPrune))

		time.Sleep(1 * time.Second)
	}
}

func (dc *DaemonConfig) runSnapshotPrune(pool string, volume *config.VolumeConfig) {
	log.Debugf("starting snapshot prune for %q", volume.VolumeName)

	driver, err := backend.NewDriver(volume.Options.Backend)
	if err != nil {
		log.Errorf("failed to get driver: %v", err)
		return
	}

	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name: strings.Join([]string{volume.PolicyName, volume.VolumeName}, "."),
			Params: storage.Params{
				"pool": pool,
			},
		},
		Timeout: dc.Timeout,
	}

	list, err := driver.ListSnapshots(driverOpts)
	if err != nil {
		log.Errorf("Could not list snapshots for volume %q: %v", volume.VolumeName, err)
		return
	}

	toDeleteCount := len(list) - int(volume.Options.Snapshot.Keep)
	if toDeleteCount < 0 {
		return
	}

	for i := 0; i < toDeleteCount; i++ {
		log.Infof("Removing snapshot %q for volume %q", list[i], volume.VolumeName)
		if err := driver.RemoveSnapshot(list[i], driverOpts); err != nil {
			log.Errorf("Removing snapshot %q for volume %q failed: %v", list[i], volume.VolumeName, err)
		}
	}
}

func (dc *DaemonConfig) runSnapshot(pool string, volume *config.VolumeConfig) {
	now := time.Now()
	log.Infof("Snapping volume %q at %v", volume, now)
	driver, err := backend.NewDriver(volume.Options.Backend)
	if err != nil {
		log.Errorf("failed to get driver: %v", err)
		return
	}
	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name: strings.Join([]string{volume.PolicyName, volume.VolumeName}, "."),
			Params: storage.Params{
				"pool": pool,
			},
		},
		Timeout: dc.Timeout,
	}

	if err := driver.CreateSnapshot(now.String(), driverOpts); err != nil {
		log.Errorf("Cannot snap volume: %q: %v", volume.VolumeName, err)
	}
}

func (dc *DaemonConfig) scheduleSnapshots() {
	for {
		log.Debug("Running snapshot supervisor")

		dc.iterateVolumes(dc.wrapSnapshotAction(dc.runSnapshot))

		time.Sleep(1 * time.Second)
	}
}
