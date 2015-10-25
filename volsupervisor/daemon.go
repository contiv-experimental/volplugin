package volsupervisor

import "github.com/contiv/volplugin/config"

// Daemon implements the startup of the various services volsupervisor manages.
// It hangs until the program terminates.
func Daemon(cfg *config.TopLevelConfig) {
	go scheduleSnapshotPrune(cfg)
	go scheduleSnapshots(cfg)
	select {}
}
