package volplugin

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage/backend/ceph"
	"github.com/gorilla/mux"
)

const basePath = "/run/docker/plugins"

// DaemonConfig is the top-level configuration for the daemon. It is used by
// the cli package in volplugin/volplugin.
type DaemonConfig struct {
	Debug   bool
	TTL     int
	Master  string
	Host    string
	Timeout time.Duration
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

	if dc.Debug {
		log.SetLevel(log.DebugLevel)
	}

	srv := http.Server{Handler: dc.configureRouter()}
	srv.SetKeepAlivesEnabled(false)
	srv.Serve(l)
	return l.Close()
}

func (dc *DaemonConfig) configureRouter() *mux.Router {
	var routeMap = map[string]func(http.ResponseWriter, *http.Request){
		"/Plugin.Activate":      activate,
		"/Plugin.Deactivate":    nilAction,
		"/VolumeDriver.Create":  create(dc.Master),
		"/VolumeDriver.Remove":  remove(dc.Master),
		"/VolumeDriver.List":    list(dc.Master),
		"/VolumeDriver.Get":     get(dc.Master),
		"/VolumeDriver.Path":    getPath(dc.Master),
		"/VolumeDriver.Mount":   mount(dc.Master, dc.Host, dc.TTL),
		"/VolumeDriver.Unmount": unmount(dc.Master, dc.Host),
	}

	router := mux.NewRouter()
	s := router.Methods("POST").Subrouter()

	for key, value := range routeMap {
		parts := strings.SplitN(key, ".", 2)
		s.HandleFunc(key, logHandler(parts[1], dc.Debug, value))
	}

	if dc.Debug {
		s.HandleFunc("{action:.*}", action)
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

func (dc *DaemonConfig) updateMounts() error {
	cd := ceph.NewDriver()
	mounts, err := cd.Mounted(dc.Timeout)
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

		volConfig, err := requestVolumeConfig(dc.Master, parts[0], parts[1])
		switch err {
		case errVolumeNotFound:
			log.Warnf("Volume %q not found in database, skipping")
			continue
		case errVolumeResponse:
			log.Fatalf("Volmaster could not be contacted; aborting volplugin.")
		}

		payload := &config.UseConfig{
			Hostname: dc.Host,
			Volume:   volConfig,
		}

		if err := reportMount(dc.Master, payload); err != nil {
			if err := reportMountStatus(dc.Master, payload); err != nil {
				// FIXME everything is effed up. what should we really be doing here?
				return err
			}
		}

		stop := addStopChan(mount.Volume.Name)
		go heartbeatMount(dc.Master, dc.TTL, payload, stop)
	}

	return nil
}
