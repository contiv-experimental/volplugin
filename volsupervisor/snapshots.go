package volsupervisor

import (
	"strings"
	"time"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend/ceph"

	log "github.com/Sirupsen/logrus"
)

func wrapSnapshotAction(action func(config *config.TopLevelConfig, pool string, volume *config.VolumeConfig)) func(*volumeDispatch) {
	return func(v *volumeDispatch) {
		for _, volume := range v.volumes {
			duration, err := time.ParseDuration(volume.Options.Snapshot.Frequency)
			if err != nil {
				log.Errorf("Runtime configuration incorrect; cannot use %q as a snapshot frequency", volume.Options.Snapshot.Frequency)
				return
			}

			if volume.Options.UseSnapshots && time.Now().Unix()%int64(duration.Seconds()) == 0 {
				action(v.config, volume.Options.Pool, volume)
			}
		}
	}
}

func scheduleSnapshotPrune(config *config.TopLevelConfig) {
	for {
		log.Debug("Running snapshot prune supervisor")

		iterateVolumes(config, wrapSnapshotAction(runSnapshotPrune))

		time.Sleep(1 * time.Second)
	}
}

func runSnapshotPrune(config *config.TopLevelConfig, pool string, volume *config.VolumeConfig) {
	log.Debugf("starting snapshot prune for %q", volume.VolumeName)

	driver := ceph.NewDriver()

	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name: strings.Join([]string{volume.TenantName, volume.VolumeName}, "."),
			Params: storage.Params{
				"pool": pool,
			},
		},
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

func runSnapshot(config *config.TopLevelConfig, pool string, volume *config.VolumeConfig) {
	now := time.Now()
	log.Infof("Snapping volume %q at %v", volume, now)
	driver := ceph.NewDriver()
	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name: strings.Join([]string{volume.TenantName, volume.VolumeName}, "."),
			Params: storage.Params{
				"pool": pool,
			},
		},
	}

	if err := driver.CreateSnapshot(now.String(), driverOpts); err != nil {
		log.Errorf("Cannot snap volume: %q: %v", volume.VolumeName, err)
	}
}

func scheduleSnapshots(config *config.TopLevelConfig) {
	for {
		log.Debug("Running snapshot supervisor")

		iterateVolumes(config, wrapSnapshotAction(runSnapshot))

		time.Sleep(1 * time.Second)
	}
}
