package volplugin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"time"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"

	log "github.com/Sirupsen/logrus"
)

func (dc *DaemonConfig) pollRuntime(volumeName string, mc *storage.Mount) {
	for {
		dc.runtimeMutex.RLock()
		stop := dc.runtimeStopChans[volumeName]
		dc.runtimeMutex.RUnlock()
		select {
		case <-stop:
			return
		case <-time.After(dc.Global.TTL):
			log.Debugf("Checking runtime parameters for volume %q (ttl: %d)", volumeName, dc.Global.TTL)

			log.Debugf("Requesting runtime configuration for volume %q", volumeName)
			resp, err := http.Get(fmt.Sprintf("http://%s/runtime/%s", dc.Master, volumeName))
			if err != nil {
				log.Error(errored.Errorf("Error processing runtime update").Combine(err))
				continue
			}

			if resp.StatusCode != 200 {
				log.Error(errored.Errorf("Error processing request for runtime configuration of volume %q: status %d received", volumeName, resp.StatusCode))
				continue
			}

			log.Debugf("Requesting runtime configuration for volume %q succeeded", volumeName)

			content, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Error(errored.Errorf("Error processing runtime update").Combine(err))
				continue
			}

			resp.Body.Close()
			var rt config.RuntimeOptions

			if err := json.Unmarshal(content, &rt); err != nil {
				log.Error(errored.Errorf("Error processing runtime update").Combine(err))
				continue
			}

			log.Debugf("Comparing runtime configuration for volume %q", volumeName)
			dc.runtimeMutex.RLock()
			existing := dc.runtimeVolumeMap[volumeName]
			dc.runtimeMutex.RUnlock()
			if !reflect.DeepEqual(rt, existing) {
				log.Debugf("Applying new cgroup rate limits for volume %q", volumeName)

				if err := applyCGroupRateLimit(rt, mc); err != nil {
					log.Error(errored.Errorf("Error processing runtime update").Combine(err))
					continue
				}

				log.Debugf("Assigning new runtime configuration for volume %q", volumeName)
				dc.runtimeMutex.Lock()
				dc.runtimeVolumeMap[volumeName] = rt
				dc.runtimeMutex.Unlock()
			}
		}
	}
}

func (dc *DaemonConfig) stopRuntimePoll(volumeName string) {
	dc.runtimeMutex.Lock()
	if stop, ok := dc.runtimeStopChans[volumeName]; ok {
		close(stop)
		delete(dc.runtimeStopChans, volumeName)
	}
	dc.runtimeMutex.Unlock()
}

func (dc *DaemonConfig) startRuntimePoll(volumeName string, mc *storage.Mount) {
	dc.runtimeMutex.Lock()
	if _, ok := dc.runtimeStopChans[volumeName]; ok {
		close(dc.runtimeStopChans[volumeName])
	}
	dc.runtimeStopChans[volumeName] = make(chan struct{})
	dc.runtimeMutex.Unlock()

	go dc.pollRuntime(volumeName, mc)
}
