package volplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/info"
	"github.com/contiv/volplugin/lock/client"
	"github.com/contiv/volplugin/storage/backend/ceph"
	"github.com/gorilla/mux"
)

const basePath = "/run/docker/plugins"

// DaemonConfig is the top-level configuration for the daemon. It is used by
// the cli package in volplugin/volplugin.
type DaemonConfig struct {
	Master string
	Host   string
	Global *config.Global
	Client *client.Driver
}

// VolumeRequest is taken from
// https://github.com/calavera/docker-volume-api/blob/master/api.go#L23
type VolumeRequest struct {
	Name string
	Opts map[string]string
}

// VolumeResponse is taken from
// https://github.com/calavera/docker-volume-api/blob/master/api.go#L23
type VolumeResponse struct {
	Mountpoint string
	Err        string
}

// volumeGet is taken from this struct in docker:
// https://github.com/docker/docker/blob/master/volume/drivers/proxy.go#L180
type volumeGet struct {
	Name string
}

// Daemon starts the volplugin service.
func (dc *DaemonConfig) Daemon() error {
	dc.Client = client.NewDriver(dc.Master)

	dc.getGlobal()

	if dc.Global == nil {
		log.Fatal("Global configuration is missing; aborting.")
	}

	if err := dc.updateMounts(); err != nil {
		return err
	}

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

	if dc.Global.Debug {
		log.SetLevel(log.DebugLevel)
	}

	go info.HandleDebugSignal()
	go dc.watchGlobal()

	srv := http.Server{Handler: dc.configureRouter()}
	srv.SetKeepAlivesEnabled(false)
	if err := srv.Serve(l); err != nil {
		log.Fatalf("Fatal error serving volplugin: %v", err)
	}
	l.Close()
	return os.Remove(driverPath)
}

func (dc *DaemonConfig) configureRouter() *mux.Router {
	var routeMap = map[string]func(http.ResponseWriter, *http.Request){
		"/Plugin.Activate":      dc.activate,
		"/Plugin.Deactivate":    dc.nilAction,
		"/VolumeDriver.Create":  dc.create,
		"/VolumeDriver.Remove":  dc.remove,
		"/VolumeDriver.List":    dc.list,
		"/VolumeDriver.Get":     dc.get,
		"/VolumeDriver.Path":    dc.getPath,
		"/VolumeDriver.Mount":   dc.mount,
		"/VolumeDriver.Unmount": dc.unmount,
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

func (dc *DaemonConfig) getGlobal() {
	resp, err := http.Get(fmt.Sprintf("http://%s/global", dc.Master))
	if err != nil {
		log.Errorf("Could not request global configuration: %v", err)
		return
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Could not request global configuration: %v", err)
		return
	}

	global := config.NewGlobalConfig()

	if err := json.Unmarshal(content, global); err != nil {
		log.Errorf("Could not request global configuration: %v", err)
		return
	}

	dc.Global = global
}

func (dc *DaemonConfig) watchGlobal() error {
	for {
		time.Sleep(1 * time.Minute)
		dc.getGlobal()
	}
}

func (dc *DaemonConfig) updateMounts() error {
	cd := ceph.NewDriver()
	mounts, err := cd.Mounted(dc.Global.Timeout)
	if err != nil {
		return err
	}

	for _, mount := range mounts {
		parts := strings.Split(mount.Volume.Name, "/")
		if len(parts) != 2 {
			log.Warnf("Invalid volume named %q in ceph scan: skipping refresh", mount.Volume.Name)
			continue
		}

		log.Infof("Refreshing existing mount for %q", mount.Volume.Name)

		volConfig, err := dc.requestVolumeConfig(parts[0], parts[1])
		switch err {
		case errVolumeNotFound:
			log.Warnf("Volume %q not found in database, skipping")
			continue
		case errVolumeResponse:
			log.Fatalf("Volmaster could not be contacted; aborting volplugin.")
		}

		payload := &config.UseMount{
			Hostname: dc.Host,
			Volume:   volConfig,
		}

		if err := dc.Client.ReportMount(payload); err != nil {
			if err := dc.Client.ReportMountStatus(payload); err != nil {
				// FIXME everything is effed up. what should we really be doing here?
				return err
			}
		}

		stop := dc.Client.AddStopChan(mount.Volume.Name)
		go dc.Client.HeartbeatMount(dc.Global.TTL, payload, stop)
	}

	return nil
}
