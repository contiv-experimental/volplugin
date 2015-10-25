package volsupervisor

import (
	"strings"
	"time"

	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"

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
	cephVol := cephdriver.NewCephDriver().NewVolume(pool, strings.Join([]string{volume.TenantName, volume.VolumeName}, "."), volume.Options.Size)
	log.Debugf("starting snapshot prune for %q", volume.VolumeName)
	list, err := cephVol.ListSnapshots()
	if err != nil {
		log.Errorf("Could not list snapshots for volume %q", volume)
		return
	}

	toDeleteCount := len(list) - int(volume.Options.Snapshot.Keep)
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

func runSnapshot(config *config.TopLevelConfig, pool string, volume *config.VolumeConfig) {
	now := time.Now()
	cephVol := cephdriver.NewCephDriver().NewVolume(pool, strings.Join([]string{volume.TenantName, volume.VolumeName}, "."), volume.Options.Size)
	log.Infof("Snapping volume %q at %v", volume, now)
	if err := cephVol.CreateSnapshot(now.String()); err != nil {
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
