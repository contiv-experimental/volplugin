package volplugin

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"golang.org/x/sys/unix"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/api"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"
	"github.com/docker/docker/pkg/plugins"
)

type unmarshalledConfig struct {
	Request api.VolumeRequest
	Name    string
	Policy  string
}

func (dc *DaemonConfig) nilAction(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(api.VolumeResponse{})
	if err != nil {
		api.DockerHTTPError(w, errors.UnmarshalRequest.Combine(err))
		return
	}
	w.Write(content)
}

func (dc *DaemonConfig) activate(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(plugins.Manifest{Implements: []string{"VolumeDriver"}})
	if err != nil {
		api.DockerHTTPError(w, errors.MarshalResponse.Combine(err))
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
		api.DockerHTTPError(w, errors.UnmarshalRequest.Combine(err))
		return nil, err
	}

	if vr.Name == "" {
		api.DockerHTTPError(w, errors.InvalidVolume.Combine(errored.New("Name is empty")))
		return nil, err
	}

	policy, name, err := splitPath(vr.Name)
	if err != nil {
		api.DockerHTTPError(w, errors.ConfiguringVolume.Combine(err))
		return nil, err
	}

	uc := unmarshalledConfig{
		Request: vr,
		Name:    name,
		Policy:  policy,
	}

	return &uc, nil
}

func writeResponse(w http.ResponseWriter, vr *api.VolumeResponse) {
	content, err := json.Marshal(*vr)
	if err != nil {
		api.DockerHTTPError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (dc *DaemonConfig) get(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		api.DockerHTTPError(w, errors.ReadBody.Combine(err))
		return
	}

	vg := volumeGetRequest{}

	if err := json.Unmarshal(content, &vg); err != nil {
		api.DockerHTTPError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	parts := strings.SplitN(vg.Name, "/", 2)

	if len(parts) < 2 {
		api.DockerHTTPError(w, errors.GetVolume.Combine(errored.Errorf("Could not parse volume %q", vg.Name)))
	}

	volConfig, err := dc.Client.GetVolume(parts[0], parts[1])
	if erd, ok := err.(*errored.Error); ok && erd.Contains(errors.NotExists) {
		w.Write([]byte("{}"))
		return
	} else if err != nil {
		api.DockerHTTPError(w, errors.GetVolume.Combine(err))
		return
	}

	content, err = json.Marshal(volConfig)
	if err != nil {
		api.DockerHTTPError(w, errors.MarshalResponse.Combine(err))
		return
	}

	if err := volConfig.Validate(); err != nil {
		api.DockerHTTPError(w, errors.ConfiguringVolume.Combine(err))
		return
	}

	driver, err := backend.NewMountDriver(volConfig.Backends.Mount, dc.Global.MountPath)
	if err != nil {
		api.DockerHTTPError(w, errors.GetDriver.Combine(err))
		return
	}

	do, err := dc.volumeToDriverOptions(volConfig)
	if err != nil {
		api.DockerHTTPError(w, errors.MarshalVolume.Combine(err))
		return
	}

	path, err := driver.MountPath(do)
	if err != nil {
		api.DockerHTTPError(w, errors.MountPath.Combine(err))
		return
	}

	content, err = json.Marshal(volumeGet{Volume: volume{Name: volConfig.String(), Mountpoint: path}})
	if err != nil {
		api.DockerHTTPError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (dc *DaemonConfig) list(w http.ResponseWriter, r *http.Request) {
	volList, err := dc.Client.ListAllVolumes()
	if err != nil {
		api.DockerHTTPError(w, errors.ListVolume.Combine(err))
		return
	}

	volumes := []*config.Volume{}
	response := volumeList{Volumes: []volume{}}

	for _, volume := range volList {
		parts := strings.SplitN(volume, "/", 2)
		if len(parts) != 2 {
			log.Errorf("")
			continue
		}
		if volObj, err := dc.Client.GetVolume(parts[0], parts[1]); err != nil {
		} else {
			volumes = append(volumes, volObj)
		}
	}

	for _, volConfig := range volumes {
		driver, err := backend.NewMountDriver(volConfig.Backends.Mount, dc.Global.MountPath)
		if err != nil {
			api.DockerHTTPError(w, errors.GetDriver.Combine(err))
			return
		}

		do, err := dc.volumeToDriverOptions(volConfig)
		if err != nil {
			api.DockerHTTPError(w, errors.MarshalVolume.Combine(err))
			return
		}

		path, err := driver.MountPath(do)
		if err != nil {
			api.DockerHTTPError(w, errors.MountPath.Combine(err))
			return
		}

		response.Volumes = append(response.Volumes, volume{Name: volConfig.String(), Mountpoint: path})
	}

	content, err := json.Marshal(response)
	if err != nil {
		api.DockerHTTPError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (dc *DaemonConfig) getPath(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		api.DockerHTTPError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	driver, _, do, err := dc.structsVolumeName(uc)
	if erd, ok := err.(*errored.Error); ok && erd.Contains(errors.NotExists) {
		log.Debugf("Volume %q not found, was requested", uc.Request.Name)
		w.Write([]byte("{}"))
		return
	} else if err != nil {
		api.DockerHTTPError(w, errors.GetDriver.Combine(err))
		return
	}

	path, err := driver.MountPath(do)
	if err != nil {
		api.DockerHTTPError(w, errors.MountPath.Combine(err))
		return
	}

	writeResponse(w, &api.VolumeResponse{Mountpoint: path})
}

func (dc *DaemonConfig) returnMountPath(w http.ResponseWriter, driver storage.MountDriver, driverOpts storage.DriverOptions) {
	path, err := driver.MountPath(driverOpts)
	if err != nil {
		api.DockerHTTPError(w, errors.MountPath.Combine(err))
		return
	}

	writeResponse(w, &api.VolumeResponse{Mountpoint: path})
}

func (dc *DaemonConfig) mount(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		api.DockerHTTPError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	log.Infof("Mounting volume %q", uc.Request.Name)

	driver, volConfig, driverOpts, err := dc.structsVolumeName(uc)
	if err != nil {
		api.DockerHTTPError(w, errors.ConfiguringVolume.Combine(err))
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

	if err := dc.Client.PublishUse(ut); err != nil && ut.Hostname != lock.Unlocked {
		api.DockerHTTPError(w, err)
		return
	}

	stopChan, err := dc.Lock.AcquireWithTTLRefresh(ut, dc.Global.TTL, dc.Global.Timeout)
	if err != nil {
		api.DockerHTTPError(w, errors.LockFailed.Combine(err))
		return
	}

	mc, err := driver.Mount(driverOpts)
	if err != nil {
		dc.decreaseMount(volName)
		api.DockerHTTPError(w, errors.MountFailed.Combine(err))
		return
	}

	if err := applyCGroupRateLimit(volConfig.RuntimeOptions, mc); err != nil {
		if dc.decreaseMount(volName) == 0 {
			if err := driver.Unmount(driverOpts); err != nil {
				log.Errorf("Could not unmount device for volume %q: %v", volName, err)
			}
		}

		api.DockerHTTPError(w, errors.RateLimit.Combine(err))
		return
	}

	dc.addMount(mc)
	dc.addStopChan(volName, stopChan)

	path, err := driver.MountPath(driverOpts)
	if err != nil {
		if dc.decreaseMount(volName) == 0 {
			if err := driver.Unmount(driverOpts); err != nil {
				log.Errorf("Could not unmount device for volume %q: %v", volName, err)
			}
		}
		dc.removeStopChan(volName)
		dc.removeMount(volName)
		api.DockerHTTPError(w, errors.MountPath.Combine(err))
		return
	}

	writeResponse(w, &api.VolumeResponse{Mountpoint: path})
}

func (dc *DaemonConfig) unmount(w http.ResponseWriter, r *http.Request) {
	uc, err := unmarshalAndCheck(w, r)
	if err != nil {
		api.DockerHTTPError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	log.Infof("Unmounting volume %q", uc.Request.Name)

	driver, volConfig, driverOpts, err := dc.structsVolumeName(uc)
	if err != nil {
		api.DockerHTTPError(w, errors.GetDriver.Combine(err))
		return
	}

	volName := volConfig.String()

	if dc.decreaseMount(volName) > 0 {
		log.Warnf("Duplicate unmount of %q detected: ignoring and returning success", volName)
		dc.returnMountPath(w, driver, driverOpts)
		return
	}

	if err := driver.Unmount(driverOpts); err != nil && err != unix.EINVAL {
		api.DockerHTTPError(w, errors.UnmountFailed.Combine(err))
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
		api.DockerHTTPError(w, errors.UnmarshalRequest.Combine(err))
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
