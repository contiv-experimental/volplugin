package volsupervisor

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/info"
	"github.com/contiv/volplugin/watch"
)

// DaemonConfig is the top-level configuration for the daemon. It is used by
// the cli package in volplugin/volplugin.
type DaemonConfig struct {
	Global   *config.Global
	Config   *config.TopLevelConfig
	Hostname string
}

// Daemon is the top-level entrypoint for the volsupervisor from the CLI.
func Daemon(ctx *cli.Context) {
	cfg, err := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err != nil {
		log.Fatal(err)
	}

retry:
	global, err := cfg.GetGlobal()
	if err != nil {
		log.Errorf("Could not retrieve global configuration: %v. Retrying in 1 second", err)
		time.Sleep(1 * time.Second)
		goto retry
	}

	dc := &DaemonConfig{Config: cfg, Global: global, Hostname: ctx.String("host-label")}
	dc.setDebug()

	globalChan := make(chan *watch.Watch)
	dc.Config.WatchGlobal(globalChan)
	go dc.watchAndSetGlobal(globalChan)
	go info.HandleDebugSignal()

	dc.updatePolicies()
	dc.loop()
}

func (dc *DaemonConfig) watchAndSetGlobal(globalChan chan *watch.Watch) {
	for {
		dc.Global = (<-globalChan).Config.(*config.Global)
		dc.setDebug()
	}
}

func (dc *DaemonConfig) setDebug() {
	if dc.Global.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled")
	} else {
		log.SetLevel(log.InfoLevel)
	}
}
