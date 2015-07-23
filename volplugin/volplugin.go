package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/librbd"
	"github.com/gorilla/mux"
)

const basePath = "/run/docker/plugins"

// VolumeRequest is taken from
// https://github.com/calavera/docker-volume-api/blob/master/api.go#L23
type VolumeRequest struct {
	Name string
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
}

// response from the volmaster
type configTenant struct {
	Pool string `json:"pool"`
	Size uint64 `json:"size"`
}

func daemon(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		fmt.Printf("\nUsage: %s [tenant/driver name]\n\n", os.Args[0])
		cli.ShowAppHelp(ctx)
		os.Exit(1)
	}

	driverName := ctx.Args()[0]
	driverPath := path.Join(basePath, driverName) + ".sock"
	os.Remove(driverPath)
	os.MkdirAll(basePath, 0700)

	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: driverPath, Net: "unix"})
	if err != nil {
		panic(err)
	}

	if ctx.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	http.Serve(l, configureRouter(driverName, ctx.Bool("debug")))
	l.Close()
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
	s := router.Headers("Accept", "application/vnd.docker.plugins.v1+json").
		Methods("POST").Subrouter()

	for key, value := range routeMap {
		parts := strings.SplitN(key, ".", 2)
		s.HandleFunc(key, logHandler(parts[1], debug, value))
	}

	if debug {
		s.HandleFunc("/VolumeDriver.{action:.*}", action)
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
