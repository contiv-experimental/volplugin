package volplugin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/sys/unix"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/lock"
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
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}
	w.Write(content)
}

func (dc *DaemonConfig) activate(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(plugins.Manifest{Implements: []string{"VolumeDriver"}})
	if err != nil {
		httpError(w, errors.MarshalResponse.Combine(err))
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
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return nil, err
	}

	if vr.Name == "" {
		httpError(w, errors.InvalidVolume.Combine(errored.New("Name is empty")))
		return nil, err
	}

	policy, name, err := splitPath(vr.Name)
	if err != nil {
		httpError(w, errors.ConfiguringVolume.Combine(err))
		return nil, err
	}

	uc := unmarshalledConfig{
		Request: vr,
		Name:    name,
		Policy:  policy,
	}

	return &uc, nil
}

func writeResponse(w http.ResponseWriter, vr *VolumeResponse) {
	content, err := marshalResponse(*vr)
	if err != nil {
		httpError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (dc *DaemonConfig) get(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, errors.ReadBody.Combine(err))
		return
	}

	vg := volumeGetRequest{}

	if err := json.Unmarshal(content, &vg); err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/volumes/%s", dc.Master, vg.Name))
	if err != nil {
		httpError(w, errors.VolmasterRequest.Combine(err))
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		w.Write([]byte("{}"))
		return
	}

	if resp.StatusCode != 200 {
		httpError(w, errors.GetVolume.Combine(errored.New(resp.Status)))
		return
	}

	volConfig := &config.Volume{}

	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		httpError(w, errors.VolmasterRequest.Combine(err))
		return
	}

	if err := json.Unmarshal(content, volConfig); err != nil {
		httpError(w, errors.UnmarshalVolume.Combine(err))
		return
	}

	if err := volConfig.Validate(); err != nil {
		httpError(w, errors.ConfiguringVolume.Combine(err))
		return
	}

	driver, err := backend.NewMountDriver(volConfig.Backends.Mount, dc.Global.MountPath)
	if err != nil {
		httpError(w, errors.GetDriver.Combine(err))
		return
	}

	do, err := dc.volumeToDriverOptions(volConfig)
	if err != nil {
		httpError(w, errors.MarshalVolume.Combine(err))
		return
	}

	path, err := driver.MountPath(do)
	if err != nil {
		httpError(w, errors.MountPath.Combine(err))
		return
	}

	content, err = json.Marshal(volumeGet{Volume: volume{Name: volConfig.String(), Mountpoint: path}})
	if err != nil {
		httpError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (dc *DaemonConfig) list(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get(fmt.Sprintf("http://%s/volumes/", dc.Master))
	if err != nil {
		httpError(w, errors.ListVolume.Combine(err))
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		httpError(w, errors.ListVolume.Combine(errored.New(resp.Status)))
		return
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		httpError(w, errors.VolmasterRequest.Combine(err))
		return
	}

	volumes := []*config.Volume{}

	if err := json.Unmarshal(content, &volumes); err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	response := volumeList{Volumes: []volume{}}

	for _, volConfig := range volumes {
		driver, err := backend.NewMountDriver(volConfig.Backends.Mount, dc.Global.MountPath)
		if err != nil {
			httpError(w, errors.GetDriver.Combine(err))
			return
		}

		do, err := dc.volumeToDriverOptions(volConfig)
		if err != nil {
			httpError(w, errors.MarshalVolume.Combine(err))
			return
		}

		path, err := driver.MountPath(do)
		if err != nil {
			httpError(w, errors.MountPath.Combine(err))
			return
		}

		response.Volumes = append(response.Volumes, volume{Name: volConfig.String(), Mountpoint: path})
	}

	content, err = json.Marshal(response)
	if err != nil {
		httpError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (dc *DaemonConfig) getPath(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	driver, _, do, err := dc.structsVolumeName(uc)
	if erd, ok := err.(*errored.Error); ok && erd.Contains(errors.NotExists) {
		log.Debugf("Volume %q not found, was requested", uc.Request.Name)
		w.Write([]byte("{}"))
		return
	} else if err != nil {
		httpError(w, errors.GetDriver.Combine(err))
		return
	}

	path, err := driver.MountPath(do)
	if err != nil {
		httpError(w, errors.MountPath.Combine(err))
		return
	}

	writeResponse(w, &VolumeResponse{Mountpoint: path})
}

func (dc *DaemonConfig) create(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	if err := dc.requestCreate(uc.Policy, uc.Name, uc.Request.Opts); err != nil {
		httpError(w, errors.GetPolicy.Combine(err))
		return
	}

	writeResponse(w, &VolumeResponse{Mountpoint: "", Err: ""})
}

func (dc *DaemonConfig) returnMountPath(w http.ResponseWriter, driver storage.MountDriver, driverOpts storage.DriverOptions) {
	path, err := driver.MountPath(driverOpts)
	if err != nil {
		httpError(w, errors.MountPath.Combine(err))
		return
	}

	writeResponse(w, &VolumeResponse{Mountpoint: path})
}

func (dc *DaemonConfig) mount(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	log.Infof("Mounting volume %q", uc.Request.Name)

	driver, volConfig, driverOpts, err := dc.structsVolumeName(uc)
	if err != nil {
		httpError(w, errors.ConfiguringVolume.Combine(err))
		return
	}

	volName := volConfig.String()

	if dc.increaseMount(volName) > 1 {
		log.Warnf("Duplicate mount of %q detected: returning existing mount path", volName)
		dc.returnMountPath(w, driver, driverOpts)
		return
	}

	ut := &config.UseMount{
		Volume:   volName,
		Reason:   lock.ReasonMount,
		Hostname: dc.Host,
	}

	if volConfig.Unlocked {
		ut.Hostname = lock.Unlocked
	}

	if err := dc.Client.ReportMount(ut); err != nil {
		httpError(w, errors.RefreshMount.Combine(err))
		return
	}

	mc, err := driver.Mount(driverOpts)
	if err != nil {
		dc.decreaseMount(volName)
		httpError(w, errors.MountFailed.Combine(err))
		if err := dc.Client.ReportUnmount(ut); err != nil {
			log.Errorf("Could not report unmount: %v", err)
		}
		return
	}

	if err := applyCGroupRateLimit(volConfig.RuntimeOptions, mc); err != nil {
		httpError(w, errors.RateLimit.Combine(err))
		return
	}

	go dc.Client.HeartbeatMount(dc.Global.TTL, ut, dc.Client.AddStopChan(uc.Request.Name))
	go dc.startRuntimePoll(volName, mc)

	path, err := driver.MountPath(driverOpts)
	if err != nil {
		httpError(w, errors.MountPath.Combine(err))
		return
	}

	writeResponse(w, &VolumeResponse{Mountpoint: path})
}

func (dc *DaemonConfig) unmount(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	log.Infof("Unmounting volume %q", uc.Request.Name)

	driver, volConfig, driverOpts, err := dc.structsVolumeName(uc)
	if err != nil {
		httpError(w, errors.GetDriver.Combine(err))
		return
	}

	volName := volConfig.String()

	if dc.decreaseMount(volName) > 0 {
		log.Warnf("Duplicate unmount of %q detected: ignoring and returning success", volName)
		dc.returnMountPath(w, driver, driverOpts)
		return
	}

	if err := driver.Unmount(driverOpts); err != nil && err != unix.EINVAL {
		httpError(w, errors.UnmountFailed.Combine(err))
		return
	}

	dc.Client.RemoveStopChan(uc.Request.Name)
	dc.stopRuntimePoll(uc.Request.Name)

	ut := &config.UseMount{
		Volume:   volName,
		Reason:   lock.ReasonMount,
		Hostname: dc.Host,
	}

	if volConfig.Unlocked {
		ut.Hostname = lock.Unlocked
	}

	if err := dc.Client.ReportUnmount(ut); err != nil {
		httpError(w, errors.RefreshMount.Combine(errored.New(volConfig.String())).Combine(err))
		return
	}

	dc.returnMountPath(w, driver, driverOpts)
}

func (dc *DaemonConfig) capabilities(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(map[string]map[string]string{
		"Capabilities": map[string]string{
			"Scope": "global",
		},
	})

	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	w.Write(content)
}

// Catchall for additional driver functions.
func (dc *DaemonConfig) action(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	log.Debugf("Unknown driver action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Debug("Body content:", string(content))
	w.WriteHeader(503)
}
