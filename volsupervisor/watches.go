package volsupervisor

import (
	"path"
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
	dc.Config.WatchVolumeRuntimes(volumeChan)

	go func() {
		for {
			runtimeConfig := <-volumeChan

			volumeMutex.Lock()
			if runtimeConfig.Config == nil {
				log.Debugf("Deleting volume %q from cache", runtimeConfig.Key)
				delete(volumes, runtimeConfig.Key)
			} else {
				log.Debugf("Adding volume %q to cache", runtimeConfig.Key)
				policy, volname := path.Split(runtimeConfig.Key)
				vol, err := dc.Config.GetVolume(policy, volname)
				if err != nil {
					log.Errorf("Could not get volume %q processing runtime update", runtimeConfig.Key)
					continue
				}

				vol.RuntimeOptions = *(runtimeConfig.Config.(*config.RuntimeOptions))
				volumes[runtimeConfig.Key] = vol
			}
			volumeMutex.Unlock()
		}
	}()

}
