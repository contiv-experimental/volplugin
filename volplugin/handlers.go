package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/cephdriver"
	"github.com/docker/docker/pkg/plugins"
)

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

func create(driver *cephdriver.CephDriver, size uint64) func(http.ResponseWriter, *http.Request) {
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
			VolumeSize: size,
		}

		log.Infof("Creating volume with parameters: %v", volSpec)

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

func getPath(driver *cephdriver.CephDriver) func(http.ResponseWriter, *http.Request) {
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
		}

		log.Infof("Returning mount path to docker for volume: %q", vr.Name)

		content, err := marshalResponse(VolumeResponse{Mountpoint: driver.MountPath(volspec.VolumeName)})
		if err != nil {
			httpError(w, "Reply could not be marshalled", err)
			return
		}

		w.Write(content)
	}
}

func mount(driver *cephdriver.CephDriver) func(http.ResponseWriter, *http.Request) {
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
		}

		log.Infof("Mounting volume %q", vr.Name)

		if err := driver.MountVolume(volspec); err != nil {
			httpError(w, "Volume could not be mounted", err)
			return
		}

		content, err := marshalResponse(VolumeResponse{Mountpoint: driver.MountPath(volspec.VolumeName)})
		if err != nil {
			httpError(w, "Reply could not be marshalled", err)
			return
		}

		w.Write(content)
	}
}

func unmount(driver *cephdriver.CephDriver) func(http.ResponseWriter, *http.Request) {
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
		}

		log.Infof("Unmounting volume %q", vr.Name)

		if err := driver.UnmountVolume(volspec); err != nil {
			httpError(w, "Could not mount image", err)
			return
		}

		content, err := marshalResponse(VolumeResponse{Mountpoint: driver.MountPath(volspec.VolumeName)})
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
