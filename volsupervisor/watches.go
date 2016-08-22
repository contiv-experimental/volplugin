package volsupervisor

import (
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/watch"
)

func (dc *DaemonConfig) signalSnapshot() {
	snapshotChan := make(chan *watch.Watch)
	dc.Config.WatchSnapshotSignal(snapshotChan)

	go func() {
		for snapshot := range snapshotChan {
			parts := strings.SplitN(snapshot.Key, "/", 2)
			if len(parts) != 2 {
				logrus.Errorf("Invalid volume name %q; please remove this signal manually.", snapshot.Key)
				continue
			}
			vol, err := dc.Config.GetVolume(parts[0], parts[1])
			if err != nil {
				logrus.Errorf("Error while fetching volume: %v", err)
				continue
			}

			go dc.createSnapshot(vol)
			if err := dc.Config.RemoveTakeSnapshot(vol.String()); err != nil {
				logrus.Errorf("Error removing snapshot reference: %v", err)
				continue
			}
		}
	}()
}
