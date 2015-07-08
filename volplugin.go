package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/cephdriver"
	"github.com/docker/docker/pkg/plugins"
	"github.com/gorilla/mux"
)

var DEBUG = os.Getenv("DEBUG")

const BASEPATH = "/usr/share/docker/plugins"

// why these types aren't in docker is beyond comprehension
// pulled from calavera's volumes api
// https://github.com/calavera/docker-volume-api/blob/master/api.go#L23

type VolumeRequest struct {
	Name string
}

type VolumeResponse struct {
	Mountpoint string
	Err        string
}

func main() {
	if len(os.Args) != 4 {
		fmt.Printf("Usage: %s [driver name] [pool name] [image size]\n", os.Args[0])
		os.Exit(1)
	}

	driverName := os.Args[1]
	poolName := os.Args[2]
	size, err := strconv.ParseUint(os.Args[3], 10, 64)
	if err != nil {
		panic(err)
	}

	driverPath := path.Join(BASEPATH, driverName) + ".sock"
	os.Remove(driverPath)

	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: driverPath, Net: "unix"})
	if err != nil {
		panic(err)
	}

	http.Serve(l, configureRouter(poolName, size))
	l.Close()
}

func configureRouter(poolName string, size uint64) *mux.Router {
	driver := cephdriver.NewCephDriver()
	router := mux.NewRouter()
	s := router.Headers("Accept", "application/vnd.docker.plugins.v1+json").
		Methods("POST").Subrouter()

	s.HandleFunc("/Plugin.Activate", activate)
	s.HandleFunc("/Plugin.Deactivate", nilAction)
	s.HandleFunc("/VolumeDriver.Create", create(driver, poolName, uint(size)))
	s.HandleFunc("/VolumeDriver.Remove", nilAction)
	s.HandleFunc("/VolumeDriver.Path", getPath(driver, poolName))
	s.HandleFunc("/VolumeDriver.Mount", mount(driver, poolName))
	s.HandleFunc("/VolumeDriver.Unmount", unmount(driver, poolName))

	if DEBUG != "" {
		s.HandleFunc("/VolumeDriver.{action:.*}", action)
	}

	return router
}

func nilAction(w http.ResponseWriter, r *http.Request) {}

func activate(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(plugins.Manifest{Implements: []string{"VolumeDriver"}})
	if err != nil {
		httpError(w, "Could not generate bootstrap response", err)
		return
	}

	w.Write(content)
}

func deactivate(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

func create(driver *cephdriver.CephDriver, poolName string, size uint) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vr, err := unmarshalRequest(r.Body)
		if err != nil {
			httpError(w, "Could not unmarshal request", err)
			return
		}

		if vr.Name == "" {
			httpError(w, "Image name is empty", nil)
			return
		}

		volSpec := cephdriver.CephVolumeSpec{
			VolumeName: vr.Name,
			PoolName:   poolName,
			VolumeSize: size,
		}

		if err := driver.CreateVolume(volSpec); err != nil {
			httpError(w, "Could not make new image", err)
			return
		}

		content, err := marshalResponse(VolumeResponse{Mountpoint: vr.Name, Err: ""})
		if err != nil {
			httpError(w, "Could not marshal response", err)
			return
		}

		w.Write(content)
	}
}

func getPath(driver *cephdriver.CephDriver, poolName string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vr, err := unmarshalRequest(r.Body)
		if err != nil {
			httpError(w, "Could not unmarshal request", err)
			return
		}

		if vr.Name == "" {
			httpError(w, "Name is empty", nil)
			return
		}

		volspec := cephdriver.CephVolumeSpec{
			VolumeName: vr.Name,
			PoolName:   poolName,
		}

		content, err := marshalResponse(VolumeResponse{Mountpoint: driver.MountPath(volspec)})
		if err != nil {
			httpError(w, "Reply could not be marshalled", err)
			return
		}

		w.Write(content)
	}
}

func mount(driver *cephdriver.CephDriver, poolName string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vr, err := unmarshalRequest(r.Body)
		if err != nil {
			httpError(w, "Could not unmarshal request", err)
			return
		}

		if vr.Name == "" {
			httpError(w, "Name is empty", nil)
			return
		}

		volspec := cephdriver.CephVolumeSpec{
			VolumeName: vr.Name,
			PoolName:   poolName,
		}

		if err := driver.MountVolume(volspec); err != nil {
			httpError(w, "Volume could not be mounted", err)
			return
		}

		content, err := marshalResponse(VolumeResponse{Mountpoint: driver.MountPath(volspec)})
		if err != nil {
			httpError(w, "Reply could not be marshalled", err)
			return
		}

		w.Write(content)
	}
}

func unmount(driver *cephdriver.CephDriver, poolName string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vr, err := unmarshalRequest(r.Body)
		if err != nil {
			httpError(w, "Could not unmarshal request", err)
			return
		}

		if vr.Name == "" {
			httpError(w, "Name is empty", nil)
			return
		}

		volspec := cephdriver.CephVolumeSpec{
			VolumeName: vr.Name,
			PoolName:   poolName,
		}

		if err := driver.UnmountVolume(volspec); err != nil {
			httpError(w, "Could not mount image", err)
			return
		}

		content, err := marshalResponse(VolumeResponse{Mountpoint: driver.MountPath(volspec)})
		if err != nil {
			httpError(w, "Reply could not be marshalled", err)
			return
		}

		w.Write(content)
	}
}

// Catchall for additional driver functions.
func action(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Unknown driver action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Debug("Body content:", string(content))
	w.WriteHeader(503)
}

func httpError(w http.ResponseWriter, message string, err error) {
	fullError := fmt.Sprintf("%s %v", message, err)

	content, errc := marshalResponse(VolumeResponse{"", fullError})
	if errc != nil {
		log.Warnf("Error received marshalling error response: %v, original error: %s", errc, fullError)
		return
	}

	log.Warnf("Returning HTTP error handling plugin negotiation: %s", fullError)
	http.Error(w, string(content), http.StatusInternalServerError)
}

func unmarshalRequest(body io.Reader) (VolumeRequest, error) {
	vr := VolumeRequest{}

	content, err := ioutil.ReadAll(body)
	if err != nil {
		return vr, err
	}

	err = json.Unmarshal(content, &vr)
	return vr, err
}

func marshalResponse(vr VolumeResponse) ([]byte, error) {
	return json.Marshal(vr)
}
