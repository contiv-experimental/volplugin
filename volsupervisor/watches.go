package volsupervisor

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/watch"
)

func (dc *DaemonConfig) updatePolicies() {
	myVolumes, err := dc.Config.ListAllVolumes()
	if err == nil {
		for _, name := range myVolumes {
			parts := strings.SplitN(name, "/", 2)
			if len(parts) < 2 {
				log.Errorf("Invalid volume %q. Skipping on apiserver startup.", name)
				continue
			}

			vol, err := dc.Config.GetVolume(parts[0], parts[1])
			if err != nil {
				log.Errorf("Could not get volume %q at apiserver startup. Skipping and waiting for update.", name)
				continue
			}
			volumeMutex.Lock()
			volumes[name] = vol
			volumeMutex.Unlock()
		}
	}

	volumeChan := make(chan *watch.Watch)
	dc.Config.WatchVolumeRuntimes(volumeChan)

	go func() {
		for {
			vc := <-volumeChan

			volumeMutex.Lock()
			if vc.Config == nil {
				log.Debugf("Deleting volume %q from cache", vc.Key)
				delete(volumes, vc.Key)
			} else {
				log.Debugf("Adding volume %q to cache", vc.Key)
				volumes[vc.Key] = vc.Config.(*config.Volume)
			}
			volumeMutex.Unlock()
		}
	}()

}

func (dc *DaemonConfig) signalSnapshot() {
	snapshotChan := make(chan *watch.Watch)
	dc.Config.WatchSnapshotSignal(snapshotChan)

	go func() {
		for snapshot := range snapshotChan {
			parts := strings.SplitN(snapshot.Key, "/", 2)
			if len(parts) != 2 {
				log.Errorf("Invalid volume name %q; please remove this signal manually.", snapshot.Key)
				continue
			}
			vol, err := dc.Config.GetVolume(parts[0], parts[1])
			if err != nil {
				log.Errorf("Error while fetching volume: %v", err)
				continue
			}

			go dc.createSnapshot(vol)
			if err := dc.Config.RemoveTakeSnapshot(vol.String()); err != nil {
				log.Errorf("Error removing snapshot reference: %v", err)
				continue
			}
		}
	}()
}
