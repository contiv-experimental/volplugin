package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/librbd"
	"github.com/gorilla/mux"
)

const basePath = "/usr/share/docker/plugins"

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

func daemon(ctx *cli.Context) {
	if len(ctx.Args()) != 3 {
		fmt.Printf("\nUsage: %s [driver name] [pool name] [image size]\n\n", os.Args[0])
		cli.ShowAppHelp(ctx)
		os.Exit(1)
	}

	driverName := ctx.Args()[0]
	poolName := ctx.Args()[1]
	size, err := strconv.ParseUint(ctx.Args()[2], 10, 64)
	if err != nil {
		panic(err)
	}

	driverPath := path.Join(basePath, driverName) + ".sock"
	os.Remove(driverPath)

	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: driverPath, Net: "unix"})
	if err != nil {
		panic(err)
	}

	if ctx.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	http.Serve(l, configureRouter(poolName, size, ctx.Bool("debug")))
	l.Close()
}

func configureRouter(poolName string, size uint64, debug bool) *mux.Router {
	config, err := librbd.ReadConfig("/etc/rbdconfig.json")
	if err != nil {
		panic(err)
	}

	driver, err := cephdriver.NewCephDriver(config, poolName)
	if err != nil {
		panic(err)
	}

	var routeMap = map[string]func(http.ResponseWriter, *http.Request){
		"/Plugin.Activate":      activate,
		"/Plugin.Deactivate":    nilAction,
		"/VolumeDriver.Create":  create(driver, size),
		"/VolumeDriver.Remove":  nilAction,
		"/VolumeDriver.Path":    getPath(driver),
		"/VolumeDriver.Mount":   mount(driver),
		"/VolumeDriver.Unmount": unmount(driver),
	}

	router := mux.NewRouter()
	s := router.Headers("Accept", "application/vnd.docker.plugins.v1+json").
		Methods("POST").Subrouter()

	for key, value := range routeMap {
		parts := strings.SplitN(key, ".", 2)
		s.HandleFunc(key, logHandler(parts[1], value))
	}

	if debug {
		s.HandleFunc("/VolumeDriver.{action:.*}", action)
	}

	return router
}

func logHandler(name string, actionFunc func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("Handling %q event", name)
		actionFunc(w, r)
	}
}
