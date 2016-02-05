package volplugin

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend/ceph"
	"github.com/docker/docker/pkg/plugins"
)

type unmarshalledConfig struct {
	Request VolumeRequest
	Name    string
	Tenant  string
}

var (
	mountStopChans = map[string]chan struct{}{}
	mountMutex     = sync.Mutex{}
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

func unmarshalAndCheck(w http.ResponseWriter, r *http.Request) (*unmarshalledConfig, error) {
	vr, err := unmarshalRequest(r.Body)
	if err != nil {
		httpError(w, "Could not unmarshal request", err)
		return nil, err
	}

	if vr.Name == "" {
		httpError(w, "Name is empty", nil)
		return nil, err
	}

	tenant, name, err := splitPath(vr.Name)
	if err != nil {
		httpError(w, "Configuring volume", err)
		return nil, err
	}

	uc := unmarshalledConfig{
		Request: vr,
		Name:    name,
		Tenant:  tenant,
	}

	return &uc, nil
}

func writeResponse(w http.ResponseWriter, r *http.Request, vr *VolumeResponse) {
	content, err := marshalResponse(*vr)
	if err != nil {
		httpError(w, "Could not marshal response", err)
		return
	}

	w.Write(content)

}

func get(master string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Retrieving volume", err)
			return
		}

		vg := volumeGet{}

		if err := json.Unmarshal(content, &vg); err != nil {
			httpError(w, "Retrieving volume", err)
			return
		}

		resp, err := http.Get(fmt.Sprintf("http://%s/get/%s", master, vg.Name))
		if err != nil {
			httpError(w, "Retrieving volume", err)
			return
		}

		if resp.StatusCode != 200 {
			httpError(w, "Retrieving volume", fmt.Errorf("Status was not 200: was %d", resp.StatusCode))
		}

		io.Copy(w, resp.Body)
	}
}

func list(master string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get(fmt.Sprintf("http://%s/list", master))
		if err != nil {
			httpError(w, "Retrieving list", err)
			return
		}

		if resp.StatusCode != 200 {
			httpError(w, "Retrieving list", fmt.Errorf("Status was not 200: was %d", resp.StatusCode))
			return
		}

		io.Copy(w, resp.Body)
	}
}

func remove(master string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		uc, err := unmarshalAndCheck(w, r)
		if err != nil {
			return
		}

		vc, err := requestVolumeConfig(master, uc.Tenant, uc.Name)
		if err != nil {
			httpError(w, "Getting volume properties", err)
			return
		}

		if vc.Options.Ephemeral {
			if err := requestRemove(master, uc.Tenant, uc.Name); err != nil {
				httpError(w, "Removing ephemeral volume", err)
				return
			}
		}

		writeResponse(w, r, &VolumeResponse{Mountpoint: ceph.MountPath(vc.Options.Pool, joinPath(uc.Tenant, uc.Name)), Err: ""})
	}
}

func create(master string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		uc, err := unmarshalAndCheck(w, r)
		if err != nil {
			return
		}

		if err := requestCreate(master, uc.Tenant, uc.Name, uc.Request.Opts); err != nil {
			httpError(w, "Could not determine tenant configuration", err)
			return
		}

		writeResponse(w, r, &VolumeResponse{Mountpoint: "", Err: ""})
	}
}

func getPath(master string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		uc, err := unmarshalAndCheck(w, r)
		if err != nil {
			return
		}

		log.Infof("Returning mount path to docker for volume: %q", uc.Request.Name)

		volConfig, err := requestVolumeConfig(master, uc.Tenant, uc.Name)
		if err != nil {
			httpError(w, "Requesting tenant configuration", err)
			return
		}

		// FIXME need to ensure that the mount exists before returning to docker
		writeResponse(w, r, &VolumeResponse{Mountpoint: ceph.MountPath(volConfig.Options.Pool, uc.Name)})
	}
}

func mount(master, host string, ttl int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		uc, err := unmarshalAndCheck(w, r)
		if err != nil {
			return
		}

		// FIXME check if we're holding the mount already
		log.Infof("Mounting volume %q", uc.Request.Name)

		volConfig, err := requestVolumeConfig(master, uc.Tenant, uc.Name)
		if err != nil {
			httpError(w, "Could not determine tenant configuration", err)
			return
		}

		driver := ceph.NewDriver()

		ut := &config.UseConfig{
			Volume:   volConfig,
			Hostname: host,
		}

		if err := reportMount(master, ut); err != nil {
			httpError(w, "Reporting mount to master", err)
			return
		}

		stopChan := addStopChan(uc.Request.Name)
		go heartbeatMount(master, ttl, ut, stopChan)

		actualSize, err := volConfig.Options.ActualSize()
		if err != nil {
			httpError(w, "Computing size of volume", err)
			return
		}

		driverOpts := storage.DriverOptions{
			Volume: storage.Volume{
				Name: joinPath(volConfig.TenantName, volConfig.VolumeName),
				Size: actualSize,
				Params: storage.Params{
					"pool": volConfig.Options.Pool,
				},
			},
			FSOptions: storage.FSOptions{
				Type: volConfig.Options.FileSystem,
			},
		}

		mc, err := driver.Mount(driverOpts)
		if err != nil {
			httpError(w, "Volume could not be mounted", err)
			return
		}

		if err := applyCGroupRateLimit(volConfig, mc); err != nil {
			httpError(w, "Applying cgroups", err)
			return
		}

		writeResponse(w, r, &VolumeResponse{Mountpoint: ceph.MountPath(volConfig.Options.Pool, joinPath(uc.Tenant, uc.Name))})
	}
}

func unmount(master, host string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		uc, err := unmarshalAndCheck(w, r)
		if err != nil {
			return
		}

		log.Infof("Unmounting volume %q", uc.Request.Name)

		volConfig, err := requestVolumeConfig(master, uc.Tenant, uc.Name)
		if err != nil {
			httpError(w, "Could not determine tenant configuration", err)
			return
		}

		driver := ceph.NewDriver()
		driverOpts := storage.DriverOptions{
			Volume: storage.Volume{
				Name: joinPath(volConfig.TenantName, volConfig.VolumeName),
				Params: storage.Params{
					"pool": volConfig.Options.Pool,
				},
			},
		}

		if err := driver.Unmount(driverOpts); err != nil {
			httpError(w, "Could not unmount image", err)
			return
		}

		ut := &config.UseConfig{
			Volume:   volConfig,
			Hostname: host,
		}

		removeStopChan(uc.Request.Name)

		if err := reportUnmount(master, ut); err != nil {
			httpError(w, "Reporting unmount to master", err)
			return
		}

		writeResponse(w, r, &VolumeResponse{Mountpoint: ceph.MountPath(volConfig.Options.Pool, joinPath(uc.Tenant, uc.Name))})
	}
}

// Catchall for additional driver functions.
func action(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Unknown driver action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Debug("Body content:", string(content))
	w.WriteHeader(503)
}
