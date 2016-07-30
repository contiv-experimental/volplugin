package api

import (
	"net/http"
	"os"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/cgroup"
	"github.com/contiv/volplugin/storage/control"

	log "github.com/Sirupsen/logrus"
)

// Create fully creates a volume
func (a *API) Create(w http.ResponseWriter, r *http.Request) {
	volume, err := a.ReadCreate(r)
	if err != nil {
		a.HTTPError(w, err)
		return
	}

	if vol, err := a.Client.GetVolume(volume.Policy, volume.Name); err == nil && vol != nil {
		a.HTTPError(w, errors.Exists)
		return
	}

	log.Infof("Creating volume %s/%s", volume)

	hostname, err := os.Hostname()
	if err != nil {
		a.HTTPError(w, errors.GetHostname.Combine(err))
		return
	}

	policyObj, err := a.Client.GetPolicy(volume.Policy)
	if err != nil {
		a.HTTPError(w, errors.GetPolicy.Combine(errored.New(volume.Policy)).Combine(err))
		return
	}

	uc := &config.UseMount{
		Volume:   volume.String(),
		Reason:   lock.ReasonCreate,
		Hostname: hostname,
	}

	snapUC := &config.UseSnapshot{
		Volume: volume.String(),
		Reason: lock.ReasonCreate,
	}

	global := *a.Global

	err = lock.NewDriver(a.Client).ExecuteWithMultiUseLock([]config.UseLocker{uc, snapUC}, global.Timeout, func(ld *lock.Driver, ucs []config.UseLocker) error {
		volConfig, err := a.Client.CreateVolume(volume)
		if err != nil {
			return err
		}

		log.Debugf("Volume Create: %#v", *volConfig)

		do, err := control.CreateVolume(policyObj, volConfig, global.Timeout)
		if err == errors.NoActionTaken {
			goto publish
		}

		if err != nil {
			return errors.CreateVolume.Combine(err)
		}

		if err := control.FormatVolume(volConfig, do); err != nil {
			if err := control.RemoveVolume(volConfig, global.Timeout); err != nil {
				log.Errorf("Error during cleanup of failed format: %v", err)
			}
			return errors.FormatVolume.Combine(err)
		}

	publish:
		if err := a.Client.PublishVolume(volConfig); err != nil && err != errors.Exists {
			if _, ok := err.(*errored.Error); !ok {
				return errors.PublishVolume.Combine(err)
			}
			return err
		}

		return a.WriteCreate(volConfig, w)
	})

	if err != nil && err != errors.Exists {
		a.HTTPError(w, errors.CreateVolume.Combine(err))
		return
	}
}

func (a *API) get(origName string, r *http.Request) (string, error) {
	policy, name, err := storage.SplitName(origName)
	if err != nil {
		return "", errors.GetVolume.Combine(err)
	}

	driver, volConfig, driverOpts, err := a.GetStorageParameters(&Volume{Policy: policy, Name: name})
	if err != nil {
		return "", errors.GetVolume.Combine(err)
	}

	if err := volConfig.Validate(); err != nil {
		return "", errors.ConfiguringVolume.Combine(err)
	}

	path, err := driver.MountPath(driverOpts)
	if err != nil {
		return "", errors.MountPath.Combine(err)
	}

	return path, nil
}

func (a *API) writePathError(w http.ResponseWriter, err error) {
	if err, ok := err.(*errored.Error); ok && err.Contains(errors.NotExists) {
		w.Write([]byte("{}"))
		return
	}
	a.HTTPError(w, err)
	return
}

func (a *API) getMountPath(driver storage.MountDriver, driverOpts storage.DriverOptions) (string, error) {
	path, err := driver.MountPath(driverOpts)
	return path, err
}

// Path is the handler for both Path and Remove requests. We do not honor
// remove requests; they can be done with volcli.
func (a *API) Path(w http.ResponseWriter, r *http.Request) {
	origName, err := a.ReadPath(r)
	if err != nil {
		a.HTTPError(w, errors.GetVolume.Combine(err))
		return
	}

	path, err := a.get(origName, r)
	if err != nil {
		a.writePathError(w, err)
		return
	}

	if err := a.WritePath(path, w); err != nil {
		a.HTTPError(w, errors.GetVolume.Combine(err))
	}
}

// Get is the request to obtain information about a volume.
func (a *API) Get(w http.ResponseWriter, r *http.Request) {
	origName, err := a.ReadGet(r)
	if err != nil {
		a.HTTPError(w, errors.GetVolume.Combine(err))
		return
	}

	path, err := a.get(origName, r)
	if err != nil {
		a.writePathError(w, err)
		return
	}

	if err := a.WriteGet(origName, path, w); err != nil {
		a.HTTPError(w, errors.GetVolume.Combine(err))
	}
}

// List is the request to obtain a list of the volumes.
func (a *API) List(w http.ResponseWriter, r *http.Request) {
	volList, err := a.Client.ListAllVolumes()
	if err != nil {
		a.HTTPError(w, errors.ListVolume.Combine(err))
		return
	}

	if err := a.WriteList(volList, w); err != nil {
		a.HTTPError(w, errors.ListVolume.Combine(err))
	}
}

// Mount is the request to mount a volume.
func (a *API) Mount(w http.ResponseWriter, r *http.Request) {
	request, err := a.ReadMount(r)
	if err != nil {
		a.HTTPError(w, errors.ConfiguringVolume.Combine(err))
		return
	}

	log.Infof("Mounting volume %q", request)
	log.Debugf("%#v", a.MountCollection)

	driver, volConfig, driverOpts, err := a.GetStorageParameters(request)
	if err != nil {
		a.HTTPError(w, errors.ConfiguringVolume.Combine(err))
		return
	}

	volName := volConfig.String()

	ut := &config.UseMount{
		Volume:   volName,
		Reason:   lock.ReasonMount,
		Hostname: a.Hostname,
	}

	if volConfig.Unlocked {
		ut.Hostname = lock.Unlocked
	}

	// XXX the only times a use lock cannot be acquired when there are no
	// previous mounts, is when in locked mode and a mount is held on another
	// host. So we take an indefinite lock HERE while we calculate whether or not
	// we already have one.
	if err := a.Client.PublishUse(ut); err != nil {
		a.HTTPError(w, errors.LockFailed.Combine(err))
		return
	}

	// XXX docker issues unmount request after every mount failure so, this evens out
	//     decreaseMount() in unmount
	if a.MountCounter.Add(volName) > 1 {
		if volConfig.Unlocked {
			log.Warnf("Duplicate mount of %q detected: returning existing mount path", volName)
			path, err := a.getMountPath(driver, driverOpts)
			if err != nil {
				a.HTTPError(w, errors.MarshalResponse.Combine(err))
				return
			}
			a.WriteMount(path, w)
			return
		}

		log.Warnf("Duplicate mount of %q detected: Lock failed", volName)
		a.HTTPError(w, errors.LockFailed.Combine(errored.Errorf("Duplicate mount")))
		return
	}

	stopChan, err := a.Lock.AcquireWithTTLRefresh(ut, (*a.Global).TTL, (*a.Global).Timeout)
	if err != nil {
		a.HTTPError(w, errors.LockFailed.Combine(err))
		return
	}

	a.AddStopChan(volName, stopChan)

	mc, err := driver.Mount(driverOpts)
	if err != nil {
		a.HTTPError(w, errors.MountFailed.Combine(err))
		return
	}

	a.MountCollection.Add(mc)

	if err := cgroup.ApplyCGroupRateLimit(volConfig.RuntimeOptions, mc); err != nil {
		log.Errorf("Could not apply cgroups to volume %q", volConfig)
	}

	path, err := driver.MountPath(driverOpts)
	if err != nil {
		a.HTTPError(w, errors.MountPath.Combine(err))
		return
	}

	a.WriteMount(path, w)
}

// Unmount is the request to unmount a volume.
func (a *API) Unmount(w http.ResponseWriter, r *http.Request) {
	request, err := a.ReadMount(r)
	if err != nil {
		a.HTTPError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	log.Infof("Unmounting volume %q", request)

	driver, volConfig, driverOpts, err := a.GetStorageParameters(request)
	if err != nil {
		a.HTTPError(w, errors.GetDriver.Combine(err))
		return
	}

	volName := volConfig.String()

	if a.MountCounter.Sub(volName) > 0 {
		log.Warnf("Duplicate unmount of %q detected: ignoring and returning success", volName)
		path, err := a.getMountPath(driver, driverOpts)
		if err != nil {
			a.HTTPError(w, errors.MarshalResponse.Combine(err))
			return
		}
		a.WriteMount(path, w)
		return
	}

	if err := driver.Unmount(driverOpts); err != nil {
		a.HTTPError(w, errors.UnmountFailed.Combine(err))
		return
	}

	a.RemoveStopChan(volName)
	a.MountCollection.Remove(volName)

	ut := &config.UseMount{
		Volume:   volName,
		Reason:   lock.ReasonMount,
		Hostname: a.Hostname,
	}

	if volConfig.Unlocked {
		ut.Hostname = lock.Unlocked
	}

	if err := a.Lock.ClearLock(ut, (*a.Global).Timeout); err != nil {
		a.HTTPError(w, errors.RefreshMount.Combine(errored.New(volConfig.String())).Combine(err))
		return
	}

	path, err := a.getMountPath(driver, driverOpts)
	if err != nil {
		a.HTTPError(w, errors.MarshalResponse.Combine(err))
		return
	}

	a.WriteMount(path, w)
}
