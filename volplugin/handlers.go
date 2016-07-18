package volplugin

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"golang.org/x/sys/unix"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/api"
	"github.com/contiv/volplugin/api/docker"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"
	"github.com/contiv/volplugin/storage/cgroup"
)

func (dc *DaemonConfig) get(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		api.DockerHTTPError(w, errors.ReadBody.Combine(err))
		return
	}

	vg := api.VolumeGetRequest{}

	if err := json.Unmarshal(content, &vg); err != nil {
		api.DockerHTTPError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	policy, name, err := storage.SplitName(vg.Name)
	if err != nil {
		api.DockerHTTPError(w, errors.GetVolume.Combine(err))
		return
	}

	volConfig, err := dc.Client.GetVolume(policy, name)
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

	do, err := volConfig.ToDriverOptions(dc.Global.Timeout)
	if err != nil {
		api.DockerHTTPError(w, errors.MarshalVolume.Combine(err))
		return
	}

	path, err := driver.MountPath(do)
	if err != nil {
		api.DockerHTTPError(w, errors.MountPath.Combine(err))
		return
	}

	content, err = json.Marshal(api.VolumeGetResponse{Volume: api.Volume{Name: volConfig.String(), Mountpoint: path}})
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
	response := api.VolumeList{Volumes: []api.Volume{}}

	for _, volume := range volList {
		policy, name, err := storage.SplitName(volume)
		if err != nil {
			log.Errorf("Invalid volume %q detected iterating volumes %v", volume, err)
			continue
		}
		if volObj, err := dc.Client.GetVolume(policy, name); err != nil {
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

		do, err := volConfig.ToDriverOptions(dc.Global.Timeout)
		if err != nil {
			api.DockerHTTPError(w, errors.MarshalVolume.Combine(err))
			return
		}

		path, err := driver.MountPath(do)
		if err != nil {
			api.DockerHTTPError(w, errors.MountPath.Combine(err))
			return
		}

		response.Volumes = append(response.Volumes, api.Volume{Name: volConfig.String(), Mountpoint: path})
	}

	content, err := json.Marshal(response)
	if err != nil {
		api.DockerHTTPError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (dc *DaemonConfig) getPath(w http.ResponseWriter, r *http.Request) {
	uc, err := docker.Unmarshal(r)
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

	docker.WriteResponse(w, &api.VolumeCreateResponse{Mountpoint: path})
}

func (dc *DaemonConfig) returnMountPath(w http.ResponseWriter, driver storage.MountDriver, driverOpts storage.DriverOptions) {
	path, err := driver.MountPath(driverOpts)
	if err != nil {
		api.DockerHTTPError(w, errors.MountPath.Combine(err))
		return
	}

	docker.WriteResponse(w, &api.VolumeCreateResponse{Mountpoint: path})
}

func (dc *DaemonConfig) mount(w http.ResponseWriter, r *http.Request) {
	uc, err := docker.Unmarshal(r)
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

	// XXX docker issues unmount request after every mount failure so, this evens out
	//     decreaseMount() in unmount
	if dc.mountCounter.Add(volName) > 1 {
		if !volConfig.Unlocked {
			log.Warnf("Duplicate mount of %q detected: Lock failed", volName)
			api.DockerHTTPError(w, errors.LockFailed.Combine(err))
			return
		}

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
		dc.mountCounter.Sub(volName)
		api.DockerHTTPError(w, errors.MountFailed.Combine(err))
		return
	}

	if err := cgroup.ApplyCGroupRateLimit(volConfig.RuntimeOptions, mc); err != nil {
		if dc.mountCounter.Sub(volName) == 0 {
			if err := driver.Unmount(driverOpts); err != nil {
				log.Errorf("Could not unmount device for volume %q: %v", volName, err)
			}
		}

		api.DockerHTTPError(w, errors.RateLimit.Combine(err))
		return
	}

	dc.mountCollection.Add(mc)
	dc.addStopChan(volName, stopChan)

	path, err := driver.MountPath(driverOpts)
	if err != nil {
		if dc.mountCounter.Sub(volName) == 0 {
			if err := driver.Unmount(driverOpts); err != nil {
				log.Errorf("Could not unmount device for volume %q: %v", volName, err)
			}
		}
		dc.removeStopChan(volName)
		dc.mountCollection.Remove(volName)
		api.DockerHTTPError(w, errors.MountPath.Combine(err))
		return
	}

	docker.WriteResponse(w, &api.VolumeCreateResponse{Mountpoint: path})
}

func (dc *DaemonConfig) unmount(w http.ResponseWriter, r *http.Request) {
	uc, err := docker.Unmarshal(r)
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

	if dc.mountCounter.Sub(volName) > 0 {
		log.Warnf("Duplicate unmount of %q detected: ignoring and returning success", volName)
		dc.returnMountPath(w, driver, driverOpts)
		return
	}

	if err := driver.Unmount(driverOpts); err != nil && err != unix.EINVAL {
		api.DockerHTTPError(w, errors.UnmountFailed.Combine(err))
		return
	}

	dc.removeStopChan(volName)
	dc.mountCollection.Remove(volName)

	ut := &config.UseMount{
		Volume:   volName,
		Reason:   lock.ReasonMount,
		Hostname: dc.Host,
	}

	if volConfig.Unlocked {
		ut.Hostname = lock.Unlocked
	}

	d := lock.NewDriver(dc.Client)

	if err := d.ClearLock(ut, 0); err != nil {
		api.DockerHTTPError(w, errors.RefreshMount.Combine(errored.New(volConfig.String())).Combine(err))
		return
	}

	dc.returnMountPath(w, driver, driverOpts)
}
