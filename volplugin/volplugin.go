package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"

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
		fmt.Printf("Usage: %s [driver name] [pool name] [image size]\n", os.Args[0])
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

	router := mux.NewRouter()
	s := router.Headers("Accept", "application/vnd.docker.plugins.v1+json").
		Methods("POST").Subrouter()

	s.HandleFunc("/Plugin.Activate", activate)
	s.HandleFunc("/Plugin.Deactivate", nilAction)
	s.HandleFunc("/VolumeDriver.Create", create(driver, size))
	s.HandleFunc("/VolumeDriver.Remove", nilAction)
	s.HandleFunc("/VolumeDriver.Path", getPath(driver))
	s.HandleFunc("/VolumeDriver.Mount", mount(driver))
	s.HandleFunc("/VolumeDriver.Unmount", unmount(driver))

	if debug {
		s.HandleFunc("/VolumeDriver.{action:.*}", action)
	}

	return router
}
