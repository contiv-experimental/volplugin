package volplugin

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"
	"github.com/docker/docker/pkg/plugins"
)

func nilAction(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(VolumeResponse{})
	if err != nil {
		httpError(w, "Could not marshal request", err)
		return
	}
	w.Write(content)
}

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

func create(master, tenantName string) func(http.ResponseWriter, *http.Request) {
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

		pool, name, err := splitPath(vr.Name)
		if err != nil {
			httpError(w, "Configuring volume", err)
			return
		}

		if err := requestCreate(master, tenantName, pool, name, vr.Opts); err != nil {
			httpError(w, "Could not determine tenant configuration", err)
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

func getPath(master string) func(http.ResponseWriter, *http.Request) {
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

		log.Infof("Returning mount path to docker for volume: %q", vr.Name)

		pool, name, err := splitPath(vr.Name)
		if err != nil {
			httpError(w, "Configuring volume", err)
			return
		}

		// FIXME need to ensure that the mount exists before returning to docker
		driver := cephdriver.NewCephDriver()

		content, err := marshalResponse(VolumeResponse{Mountpoint: driver.MountPath(pool, name)})
		if err != nil {
			httpError(w, "Reply could not be marshalled", err)
			return
		}

		w.Write(content)
	}
}

func mount(master, host string) func(http.ResponseWriter, *http.Request) {
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

		// FIXME check if we're holding the mount already
		log.Infof("Mounting volume %q", vr.Name)

		pool, name, err := splitPath(vr.Name)
		if err != nil {
			httpError(w, "Configuring volume", err)
			return
		}

		tenConfig, err := requestTenantConfig(master, pool, name)
		if err != nil {
			httpError(w, "Could not determine tenant configuration", err)
			return
		}

		driver := cephdriver.NewCephDriver()

		mt := &config.MountConfig{
			Volume:     name,
			Pool:       pool,
			MountPoint: driver.MountPath(pool, name),
			Host:       host,
		}

		if err := reportMount(master, mt); err != nil {
			httpError(w, "Reporting mount to master", err)
			return
		}

		if err := driver.NewVolume(pool, name, tenConfig.Options.Size).Mount(); err != nil {
			httpError(w, "Volume could not be mounted", err)
			return
		}

		content, err := marshalResponse(VolumeResponse{Mountpoint: driver.MountPath(pool, name)})
		if err != nil {
			httpError(w, "Reply could not be marshalled", err)
			return
		}

		w.Write(content)
	}
}

func unmount(master string) func(http.ResponseWriter, *http.Request) {
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

		log.Infof("Unmounting volume %q", vr.Name)

		pool, name, err := splitPath(vr.Name)
		if err != nil {
			httpError(w, "Configuring volume", err)
			return
		}

		tenConfig, err := requestTenantConfig(master, pool, name)
		if err != nil {
			httpError(w, "Could not determine tenant configuration", err)
			return
		}

		driver := cephdriver.NewCephDriver()

		if err := driver.NewVolume(pool, name, tenConfig.Options.Size).Unmount(); err != nil {
			httpError(w, "Could not unmount image", err)
			return
		}

		hostname, err := os.Hostname()
		if err != nil {
			httpError(w, "Retrieving hostname", err)
			return
		}

		mt := &config.MountConfig{
			Volume:     name,
			MountPoint: driver.MountPath(pool, name),
			Pool:       pool,
			Host:       hostname,
		}

		if err := reportUnmount(master, mt); err != nil {
			httpError(w, "Reporting unmount to master", err)
			return
		}

		content, err := marshalResponse(VolumeResponse{Mountpoint: driver.MountPath(pool, name)})
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
