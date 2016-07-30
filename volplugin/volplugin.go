package volplugin

import (
	"net"
	"net/http"
	"os"
	"path"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/api"
	"github.com/contiv/volplugin/api/impl/docker"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/info"
	"github.com/contiv/volplugin/watch"
	"github.com/jbeda/go-wait"
)

const basePath = "/run/docker/plugins"

// DaemonConfig is the top-level configuration for the daemon. It is used by
// the cli package in volplugin/volplugin.
type DaemonConfig struct {
	Hostname string
	Global   *config.Global
	Client   *config.Client
	API      *api.API
}

// NewDaemonConfig creates a DaemonConfig from the master host and hostname
// arguments.
func NewDaemonConfig(ctx *cli.Context) *DaemonConfig {

retry:
	client, err := config.NewClient(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err != nil {
		log.Warn("Could not establish client to etcd cluster: %v. Retrying.", err)
		time.Sleep(wait.Jitter(time.Second, 0))
		goto retry
	}

	return &DaemonConfig{
		Hostname: ctx.String("host-label"),
		Client:   client,
	}
}

// Daemon starts the volplugin service.
func (dc *DaemonConfig) Daemon() error {
	global, err := dc.Client.GetGlobal()
	if err != nil {
		log.Errorf("Error fetching global configuration: %v", err)
		log.Infof("No global configuration. Proceeding with defaults...")
		global = config.NewGlobalConfig()
	}

	dc.Global = global
	errored.AlwaysDebug = dc.Global.Debug
	errored.AlwaysTrace = dc.Global.Debug
	if dc.Global.Debug {
		log.SetLevel(log.DebugLevel)
	}

	go info.HandleDebugSignal()

	activity := make(chan *watch.Watch)
	dc.Client.WatchGlobal(activity)
	go func() {
		for {
			dc.Global = (<-activity).Config.(*config.Global)

			log.Debugf("Received global %#v", dc.Global)

			errored.AlwaysDebug = dc.Global.Debug
			errored.AlwaysTrace = dc.Global.Debug
			if dc.Global.Debug {
				log.SetLevel(log.DebugLevel)
			}
		}
	}()

	dc.API = api.NewAPI(docker.NewVolplugin(), dc.Hostname, dc.Client, &dc.Global)

	if err := dc.updateMounts(); err != nil {
		return err
	}

	go dc.pollRuntime()

	driverPath := path.Join(basePath, "volplugin.sock")
	if err := os.Remove(driverPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return err
	}

	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: driverPath, Net: "unix"})
	if err != nil {
		return err
	}

	srv := http.Server{Handler: dc.API.Router(dc.API)}
	srv.SetKeepAlivesEnabled(false)
	if err := srv.Serve(l); err != nil {
		log.Fatalf("Fatal error serving volplugin: %v", err)
	}
	l.Close()
	return os.Remove(driverPath)
}
