package volplugin

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/librbd"
	"github.com/gorilla/mux"
)

const basePath = "/run/docker/plugins"

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

// request to the volmaster
type request struct {
	Tenant string `json:"tenant"`
	Volume string `json:"volume"`
}

type createRequest struct {
	Tenant string `json:"tenant"`
	Volume string `json:"volume"`
}

// response from the volmaster
type configTenant struct {
	Pool string `json:"pool"`
	Size uint64 `json:"size"`
}

// Daemon starts the volplugin service.
func Daemon(tenantName string, debug bool) error {
	driverPath := path.Join(basePath, tenantName) + ".sock"
	os.Remove(driverPath)
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return err
	}

	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: driverPath, Net: "unix"})
	if err != nil {
		return err
	}

	if debug {
		log.SetLevel(log.DebugLevel)
	}

	http.Serve(l, configureRouter(tenantName, debug))
	return l.Close()
}

func configureRouter(tenant string, debug bool) *mux.Router {
	config, err := librbd.ReadConfig("/etc/rbdconfig.json")
	if err != nil {
		panic(err)
	}

	var routeMap = map[string]func(http.ResponseWriter, *http.Request){
		"/Plugin.Activate":      activate,
		"/Plugin.Deactivate":    nilAction,
		"/VolumeDriver.Create":  create(tenant, config),
		"/VolumeDriver.Remove":  nilAction,
		"/VolumeDriver.Path":    getPath(tenant, config),
		"/VolumeDriver.Mount":   mount(tenant, config),
		"/VolumeDriver.Unmount": unmount(tenant, config),
	}

	router := mux.NewRouter()
	s := router.Methods("POST").Subrouter()

	for key, value := range routeMap {
		parts := strings.SplitN(key, ".", 2)
		s.HandleFunc(key, logHandler(parts[1], debug, value))
	}

	if debug {
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
