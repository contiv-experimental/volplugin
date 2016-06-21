package volplugin

import (
	"fmt"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/watch"

	log "github.com/Sirupsen/logrus"
)

func (dc *DaemonConfig) pollRuntime() {
	volumeChan := make(chan *watch.Watch)
	dc.Client.WatchVolumeRuntimes(volumeChan)
	for {
		volWatch := <-volumeChan

		fmt.Printf("%#v\n", volWatch)

		if volWatch.Config == nil {
			continue
		}

		var vol *config.Volume
		var ok bool

		if vol, ok = volWatch.Config.(*config.Volume); !ok {
			log.Error(errored.Errorf("Error processing runtime update for volume %q: assertion failed", vol))
			continue
		}

		log.Debugf("Processing volume %q", vol)
		thisMC, err := dc.getMount(vol.String())
		if err != nil {
			log.Errorf("Error retrieving mount information for %q from cache: %v", vol, err)
			continue
		}

		// if we can't look it up, it's possible it was mounted on a different host.
		if er, ok := err.(*errored.Error); ok && !er.Contains(errors.NotExists) {
			log.Errorf("Unknown error processing runtime configuration parameters for volume %q: %v", vol, er)
			continue
		}

		if err := applyCGroupRateLimit(vol.RuntimeOptions, thisMC); err != nil {
			log.Error(errored.Errorf("Error processing runtime update for volume %q", vol).Combine(err))
			continue
		}
	}
}
