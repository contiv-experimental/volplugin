package volplugin

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"
	"github.com/docker/docker/pkg/plugins"
)

type unmarshalledConfig struct {
	Request VolumeRequest
	Name    string
	Policy  string
}

func (dc *DaemonConfig) nilAction(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(VolumeResponse{})
	if err != nil {
		httpError(w, "Could not marshal request", err)
		return
	}
	w.Write(content)
}

func (dc *DaemonConfig) activate(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(plugins.Manifest{Implements: []string{"VolumeDriver"}})
	if err != nil {
		httpError(w, "Could not generate bootstrap response", err)
		return
	}

	w.Write(content)
}

func (dc *DaemonConfig) deactivate(w http.ResponseWriter, r *http.Request) {
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

	policy, name, err := splitPath(vr.Name)
	if err != nil {
		httpError(w, "Configuring volume", err)
		return nil, err
	}

	uc := unmarshalledConfig{
		Request: vr,
		Name:    name,
		Policy:  policy,
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

func (dc *DaemonConfig) get(w http.ResponseWriter, r *http.Request) {
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

	resp, err := http.Get(fmt.Sprintf("http://%s/get/%s", dc.Master, vg.Name))
	if err != nil {
		httpError(w, "Retrieving volume", err)
		return
	}

	if resp.StatusCode != 200 {
		httpError(w, "Retrieving volume", fmt.Errorf("Status was not 200: was %d", resp.StatusCode))
	}

	io.Copy(w, resp.Body)
}

func (dc *DaemonConfig) list(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get(fmt.Sprintf("http://%s/list", dc.Master))
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

func (dc *DaemonConfig) remove(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		return
	}

	vc, err := dc.requestVolumeConfig(uc.Policy, uc.Name)
	if err != nil {
		httpError(w, "Getting volume properties", err)
		return
	}

	if vc.Options.Ephemeral {
		if err := dc.requestRemove(uc.Policy, uc.Name); err != nil {
			httpError(w, "Removing ephemeral volume", err)
			return
		}
	}

	driver, err := backend.NewDriver(vc.Options.Backend, dc.Global.MountPath)
	if err != nil {
		httpError(w, fmt.Sprintf("loading driver"), err)
		return
	}

	name, err := driver.InternalName(uc.Request.Name)
	if err != nil {
		httpError(w, fmt.Sprintf("Removing volume %q", uc.Request.Name), err)
	}

	do := storage.DriverOptions{
		Volume: storage.Volume{
			Name: name,
			Params: storage.Params{
				"pool": vc.Options.Pool,
			},
		},
	}

	writeResponse(w, r, &VolumeResponse{Mountpoint: driver.MountPath(do), Err: ""})
}

func (dc *DaemonConfig) create(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		return
	}

	if err := dc.requestCreate(uc.Policy, uc.Name, uc.Request.Opts); err != nil {
		httpError(w, "Could not determine policy configuration", err)
		return
	}

	writeResponse(w, r, &VolumeResponse{Mountpoint: "", Err: ""})
}

func (dc *DaemonConfig) getPath(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		return
	}

	log.Infof("Returning mount path to docker for volume: %q", uc.Request.Name)

	volConfig, err := dc.requestVolumeConfig(uc.Policy, uc.Name)
	if err != nil {
		httpError(w, "Requesting policy configuration", err)
		return
	}

	driver, err := backend.NewDriver(volConfig.Options.Backend, dc.Global.MountPath)
	if err != nil {
		httpError(w, fmt.Sprintf("loading driver"), err)
		return
	}

	name, err := driver.InternalName(uc.Request.Name)
	if err != nil {
		httpError(w, fmt.Sprintf("Removing volume %q", uc.Request.Name), err)
	}

	do := storage.DriverOptions{
		Volume: storage.Volume{
			Name: name,
			Params: storage.Params{
				"pool": volConfig.Options.Pool,
			},
		},
	}

	// FIXME need to ensure that the mount exists before returning to docker
	writeResponse(w, r, &VolumeResponse{Mountpoint: driver.MountPath(do)})
}

func (dc *DaemonConfig) mount(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		return
	}

	// FIXME check if we're holding the mount already
	log.Infof("Mounting volume %q", uc.Request.Name)

	volConfig, err := dc.requestVolumeConfig(uc.Policy, uc.Name)
	if err != nil {
		httpError(w, "Could not determine policy configuration", err)
		return
	}

	driver, err := backend.NewDriver(volConfig.Options.Backend, dc.Global.MountPath)
	if err != nil {
		httpError(w, fmt.Sprintf("loading driver"), err)
		return
	}

	ut := &config.UseMount{
		Volume:   volConfig,
		Hostname: dc.Host,
	}

	if err := dc.Client.ReportMount(ut); err != nil {
		httpError(w, "Reporting mount to master", err)
		return
	}

	stopChan := dc.Client.AddStopChan(uc.Request.Name)
	go dc.Client.HeartbeatMount(dc.Global.TTL, ut, stopChan)

	actualSize, err := volConfig.Options.ActualSize()
	if err != nil {
		dc.Client.RemoveStopChan(uc.Request.Name)
		httpError(w, "Computing size of volume", err)
		return
	}

	intName, err := driver.InternalName(uc.Request.Name)
	if err != nil {
		httpError(w, fmt.Sprintf("Volume %q does not satisfy name requirements", uc.Request.Name), err)
		return
	}

	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name: intName,
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
		dc.Client.RemoveStopChan(uc.Request.Name)
		httpError(w, "Volume could not be mounted", err)
		return
	}

	if err := applyCGroupRateLimit(volConfig, mc); err != nil {
		dc.Client.RemoveStopChan(uc.Request.Name)
		httpError(w, "Applying cgroups", err)
		return
	}

	writeResponse(w, r, &VolumeResponse{Mountpoint: driver.MountPath(driverOpts)})
}

func (dc *DaemonConfig) unmount(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		return
	}

	log.Infof("Unmounting volume %q", uc.Request.Name)

	volConfig, err := dc.requestVolumeConfig(uc.Policy, uc.Name)
	if err != nil {
		httpError(w, "Could not determine policy configuration", err)
		return
	}

	driver, err := backend.NewDriver(volConfig.Options.Backend, dc.Global.MountPath)
	if err != nil {
		httpError(w, fmt.Sprintf("loading driver"), err)
		return
	}

	intName, err := driver.InternalName(uc.Request.Name)
	if err != nil {
		httpError(w, fmt.Sprintf("Volume %q does not satisfy name requirements", uc.Request.Name), err)
		return
	}

	driverOpts := storage.DriverOptions{
		Volume: storage.Volume{
			Name: intName,
			Params: storage.Params{
				"pool": volConfig.Options.Pool,
			},
		},
	}

	ut := &config.UseMount{
		Volume:   volConfig,
		Hostname: dc.Host,
	}

	dc.Client.RemoveStopChan(uc.Request.Name)

	if err := driver.Unmount(driverOpts); err != nil {
		dc.Client.AddStopChan(uc.Request.Name)
		httpError(w, "Could not unmount image", err)
		return
	}

	if err := dc.Client.ReportUnmount(ut); err != nil {
		httpError(w, "Reporting unmount to master", err)
		return
	}

	writeResponse(w, r, &VolumeResponse{Mountpoint: driver.MountPath(driverOpts)})
}

// Catchall for additional driver functions.
func (dc *DaemonConfig) action(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Unknown driver action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Debug("Body content:", string(content))
	w.WriteHeader(503)
}
