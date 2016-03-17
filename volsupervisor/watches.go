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
				log.Errorf("Invalid volume %q. Skipping on volmaster startup.", name)
				continue
			}

			vol, err := dc.Config.GetVolume(parts[0], parts[1])
			if err != nil {
				log.Errorf("Could not get volume %q at volmaster startup. Skipping and waiting for update.", name)
				continue
			}
			volumeMutex.Lock()
			volumes[name] = vol
			volumeMutex.Unlock()
		}
	}

	volumeChan := make(chan *watch.Watch)
	dc.Config.WatchVolumes(volumeChan)

	go func() {
		for {
			volume := <-volumeChan

			volumeMutex.Lock()
			if volume.Config == nil {
				log.Debugf("Deleting volume %q from cache", volume.Key)
				delete(volumes, volume.Key)
			} else {
				log.Debugf("Adding volume %q to cache", volume.Key)
				volumes[volume.Key] = volume.Config.(*config.Volume)
			}
			volumeMutex.Unlock()
		}
	}()

}
