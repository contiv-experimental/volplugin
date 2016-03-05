package volsupervisor

import (
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/info"
)

// DaemonConfig is the top-level configuration for the daemon. It is used by
// the cli package in volplugin/volplugin.
type DaemonConfig struct {
	Global *config.Global
	Config *config.TopLevelConfig
}

func (dc *DaemonConfig) setDebug() {
	if dc.Global.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled")
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

// Daemon implements the startup of the various services volsupervisor manages.
// It hangs until the program terminates.
func (dc *DaemonConfig) Daemon() {
	var err error

	dc.Global, err = dc.Config.GetGlobal()
	if err != nil {
		log.Fatalf("Could not retrieve global configuration: %v", err)
	}

	dc.setDebug()

	globalChan := make(chan *config.Global)
	go dc.Config.WatchGlobal(globalChan)
	go func() {
		for {
			dc.Global = <-globalChan
			dc.setDebug()
		}
	}()

	go info.HandleDebugSignal()
	go dc.scheduleSnapshotPrune()
	go dc.scheduleSnapshots()
	select {}
}
