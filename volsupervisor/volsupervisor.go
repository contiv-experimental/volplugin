package volsupervisor

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/info"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/watch"
)

// DaemonConfig is the top-level configuration for the daemon. It is used by
// the cli package in volplugin/volplugin.
type DaemonConfig struct {
	Global   *config.Global
	Config   *config.Client
	Hostname string
}

// Daemon is the top-level entrypoint for the volsupervisor from the CLI.
func Daemon(ctx *cli.Context) {
	cfg, err := config.NewClient(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err != nil {
		logrus.Fatal(err)
	}

retry:
	global, err := cfg.GetGlobal()
	if err != nil {
		logrus.Errorf("Could not retrieve global configuration: %v. Retrying in 1 second", err)
		time.Sleep(time.Second)
		goto retry
	}

	dc := &DaemonConfig{Config: cfg, Global: global, Hostname: ctx.String("host-label")}
	dc.setDebug()

	globalChan := make(chan *watch.Watch)
	dc.Config.WatchGlobal(globalChan)
	go dc.watchAndSetGlobal(globalChan)
	go info.HandleDebugSignal()

	stopChan, err := lock.NewDriver(dc.Config).AcquireWithTTLRefresh(&config.UseVolsupervisor{Hostname: dc.Hostname}, dc.Global.TTL, dc.Global.Timeout)
	if err != nil {
		logrus.Fatal("Could not start volsupervisor: already in use")
	}

	sigChan := make(chan os.Signal, 1)

	go func() {
		<-sigChan
		logrus.Infof("Removing volsupervisor global lock; waiting %v for lock to clear", dc.Global.TTL)
		stopChan <- struct{}{}
		time.Sleep(dc.Global.TTL + 1*time.Second) // give us enough time to try to clear the lock
		os.Exit(0)
	}()

	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	dc.signalSnapshot()
	dc.updateVolumes()
	// doing it here ensures the goroutine is created when the first poll completes.
	go func() {
		for {
			time.Sleep(1 * time.Second)
			dc.updateVolumes()
		}
	}()

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
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("Debug logging enabled")
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}
