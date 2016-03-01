package volsupervisor

import (
	"time"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/info"
)

// DaemonConfig is the top-level configuration for the daemon. It is used by
// the cli package in volplugin/volplugin.
type DaemonConfig struct {
	Debug   bool
	Timeout time.Duration
	Config  *config.TopLevelConfig
}

// Daemon implements the startup of the various services volsupervisor manages.
// It hangs until the program terminates.
func (dc *DaemonConfig) Daemon() {
	go info.HandleDebugSignal()
	go dc.scheduleSnapshotPrune()
	go dc.scheduleSnapshots()
	select {}
}
