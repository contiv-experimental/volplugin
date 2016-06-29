package volplugin

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/api"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/info"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/watch"
	"github.com/gorilla/mux"
	"github.com/jbeda/go-wait"
)

const basePath = "/run/docker/plugins"

// DaemonConfig is the top-level configuration for the daemon. It is used by
// the cli package in volplugin/volplugin.
type DaemonConfig struct {
	Host   string
	Lock   *lock.Driver
	Global *config.Global
	Client *config.Client

	lockStopChanMutex sync.Mutex
	lockStopChans     map[string]chan struct{}
	mountCountMutex   sync.Mutex
	mountCount        map[string]int
	mountMap          map[string]*storage.Mount
	mountMapMutex     sync.Mutex
}

// volumeGet is taken from this struct in docker:
// https://github.com/docker/docker/blob/master/volume/drivers/proxy.go#L180
type volumeGetRequest struct {
	Name string
}

type volume struct {
	Name       string
	Mountpoint string
}

type volumeList struct {
	Volumes []volume
	Err     string
}

// volumeGet is taken from this struct in docker:
// https://github.com/docker/docker/blob/master/volume/drivers/proxy.go#L180
type volumeGet struct {
	Volume volume
	Err    string
}

// NewDaemonConfig creates a DaemonConfig from the master host and hostname
// arguments.
func NewDaemonConfig(ctx *cli.Context) *DaemonConfig {

retry:
	client, err := config.NewClient(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err != nil {
		log.Warn("Could not establish client to etcd cluster: %v. Retrying.", err)
		time.Sleep(wait.Jitter(1*time.Second, 0))
		goto retry
	}

	driver := lock.NewDriver(client)

	return &DaemonConfig{
		Host:   ctx.String("host-label"),
		Client: client,
		Lock:   driver,

		lockStopChans: map[string]chan struct{}{},
		mountCount:    map[string]int{},
		mountMap:      map[string]*storage.Mount{},
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

	srv := http.Server{Handler: dc.configureRouter()}
	srv.SetKeepAlivesEnabled(false)
	if err := srv.Serve(l); err != nil {
		log.Fatalf("Fatal error serving volplugin: %v", err)
	}
	l.Close()
	return os.Remove(driverPath)
}

func (dc *DaemonConfig) configureRouter() *mux.Router {
	api := api.NewAPI(dc.Client, &dc.Global, true)

	var routeMap = map[string]func(http.ResponseWriter, *http.Request){
		"/Plugin.Activate":           dc.activate,
		"/Plugin.Deactivate":         dc.nilAction,
		"/VolumeDriver.Create":       api.Create,
		"/VolumeDriver.Remove":       dc.getPath, // we never actually remove through docker's interface.
		"/VolumeDriver.List":         dc.list,
		"/VolumeDriver.Get":          dc.get,
		"/VolumeDriver.Path":         dc.getPath,
		"/VolumeDriver.Mount":        dc.mount,
		"/VolumeDriver.Unmount":      dc.unmount,
		"/VolumeDriver.Capabilities": dc.capabilities,
	}

	router := mux.NewRouter()
	s := router.Methods("POST").Subrouter()

	for key, value := range routeMap {
		parts := strings.SplitN(key, ".", 2)
		s.HandleFunc(key, logHandler(parts[1], dc.Global.Debug, value))
	}

	if dc.Global.Debug {
		s.HandleFunc("{action:.*}", dc.action)
	}

	return router
}

func logHandler(name string, debug bool, actionFunc func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if debug {
			buf := new(bytes.Buffer)
			io.Copy(buf, r.Body)
			log.Debugf("Dispatching %s with %v", name, strings.TrimSpace(string(buf.Bytes())))
			var writer *io.PipeWriter
			r.Body, writer = io.Pipe()
			go func() {
				io.Copy(writer, buf)
				writer.Close()
			}()
		}

		actionFunc(w, r)
	}
}
