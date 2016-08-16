package volplugin

import (
	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage/cgroup"
	"github.com/contiv/volplugin/watch"
)

func (dc *DaemonConfig) pollRuntime() {
	volumeChan := make(chan *watch.Watch)
	dc.Client.WatchVolumeRuntimes(volumeChan)
	for {
		volWatch := <-volumeChan

		if volWatch.Config == nil {
			continue
		}

		var vol *config.Volume
		var ok bool

		if vol, ok = volWatch.Config.(*config.Volume); !ok {
			logrus.Error(errored.Errorf("Error processing runtime update for volume %q: assertion failed", vol))
			continue
		}

		logrus.Infof("Adjusting runtime parameters for volume %q", vol)
		thisMC, err := dc.API.MountCollection.Get(vol.String())

		if er, ok := err.(*errored.Error); ok && !er.Contains(errors.NotExists) {
			logrus.Errorf("Unknown error processing runtime configuration parameters for volume %q: %v", vol, er)
			continue
		}

		// if we can't look it up, it's possible it was mounted on a different host.
		if err != nil {
			logrus.Errorf("Error retrieving mount information for %q from cache: %v", vol, err)
			continue
		}

		if err := cgroup.ApplyCGroupRateLimit(vol.RuntimeOptions, thisMC); err != nil {
			logrus.Error(errored.Errorf("Error processing runtime update for volume %q", vol).Combine(err))
			continue
		}
	}
}
