package volplugin

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
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
	defer r.Body.Close()
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
	defer r.Body.Close()

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

	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		w.Write([]byte("{}"))
		return
	}

	if resp.StatusCode != 200 {
		httpError(w, "Retrieving volume", errored.Errorf("Status was not 200: was %d", resp.StatusCode))
		return
	}

	io.Copy(w, resp.Body)
}

func (dc *DaemonConfig) list(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get(fmt.Sprintf("http://%s/list", dc.Master))
	if err != nil {
		httpError(w, "Retrieving list", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		httpError(w, "Retrieving list", errored.Errorf("Status was not 200: was %d", resp.StatusCode))
		return
	}

	io.Copy(w, resp.Body)
}

func (dc *DaemonConfig) getPath(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		return
	}

	driver, _, do, err := dc.structsVolumeName(uc)
	if err != nil {
		httpError(w, "Configuring request", err)
		return
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

func (dc *DaemonConfig) mount(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		return
	}

	driver, volConfig, driverOpts, err := dc.structsVolumeName(uc)
	if err != nil {
		httpError(w, "Configuring request", err)
		return
	}

	log.Infof("Mounting volume %q", uc.Request.Name)

	// if we're mounted already on this host, the mount publish will succeed and
	// we will have two mounts, which will cause trouble at unmount time.

	exists, err := dc.mountExists(driver, driverOpts)
	if err != nil {
		httpError(w, "Mountpoint existence check", err)
		return
	}

	if exists {
		httpError(w, "Mountpoint already in use", err)
		return
	}

	ut := &config.UseMount{
		Volume:   volConfig.String(),
		Hostname: dc.Host,
	}

	if err := dc.Client.ReportMount(ut); err != nil {
		httpError(w, "Reporting mount to master", err)
		return
	}

	mc, err := driver.Mount(driverOpts)
	if err != nil {
		httpError(w, "Volume could not be mounted", err)
		if err := dc.Client.ReportUnmount(ut); err != nil {
			log.Errorf("Could not report unmount: %v", err)
		}
		return
	}

	if err := applyCGroupRateLimit(volConfig.RuntimeOptions, mc); err != nil {
		httpError(w, "Applying cgroups", err)
		return
	}

	go dc.Client.HeartbeatMount(dc.Global.TTL, ut, dc.Client.AddStopChan(uc.Request.Name))

	go dc.startRuntimePoll(volConfig.String(), mc)
	writeResponse(w, r, &VolumeResponse{Mountpoint: driver.MountPath(driverOpts)})
}

func (dc *DaemonConfig) unmount(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		return
	}

	log.Infof("Unmounting volume %q", uc.Request.Name)

	driver, volConfig, driverOpts, err := dc.structsVolumeName(uc)
	if err != nil {
		httpError(w, "Configuring request", err)
		return
	}

	ut := &config.UseMount{
		Volume:   volConfig.String(),
		Hostname: dc.Host,
	}

	if err := driver.Unmount(driverOpts); err != nil {
		httpError(w, "Could not unmount image", err)
		return
	}

	dc.Client.RemoveStopChan(uc.Request.Name)
	dc.stopRuntimePoll(uc.Request.Name)

	if err := dc.Client.ReportUnmount(ut); err != nil {
		httpError(w, fmt.Sprintf("Reporting unmount for volume %v, to master", volConfig), err)
		return
	}

	writeResponse(w, r, &VolumeResponse{Mountpoint: driver.MountPath(driverOpts)})
}

// Catchall for additional driver functions.
func (dc *DaemonConfig) action(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	log.Debugf("Unknown driver action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Debug("Body content:", string(content))
	w.WriteHeader(503)
}
