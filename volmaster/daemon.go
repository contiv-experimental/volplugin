package volmaster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/info"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"
	"github.com/contiv/volplugin/watch"
	"github.com/coreos/etcd/client"
	"github.com/gorilla/mux"
)

// DaemonConfig is the configuration struct used by the volmaster to hold globals.
type DaemonConfig struct {
	Config   *config.Client
	MountTTL int
	Timeout  time.Duration
	Global   *config.Global
}

// volume is the json response of a volume. Taken from
// https://github.com/docker/docker/blob/master/volume/drivers/adapter.go#L75
type volume struct {
	Name       string
	Mountpoint string
}

type routeHandlers map[string]func(http.ResponseWriter, *http.Request)

// Daemon initializes the daemon for use.
func (d *DaemonConfig) Daemon(listen string) {
	global, err := d.Config.GetGlobal()
	if err != nil {
		log.Errorf("Error fetching global configuration: %v", err)
		log.Infof("No global configuration. Proceeding with defaults...")
		global = config.NewGlobalConfig()
	}

	d.Global = global
	if d.Global.Debug {
		log.SetLevel(log.DebugLevel)
	}
	errored.AlwaysDebug = d.Global.Debug
	errored.AlwaysTrace = d.Global.Debug

	go info.HandleDebugSignal()
	go info.HandleDumpTarballSignal(d.Config)

	activity := make(chan *watch.Watch)
	d.Config.WatchGlobal(activity)
	go func() {
		for {
			d.Global = (<-activity).Config.(*config.Global)

			errored.AlwaysDebug = d.Global.Debug
			errored.AlwaysTrace = d.Global.Debug
		}
	}()

	r := mux.NewRouter()

	postRouter := map[string]func(http.ResponseWriter, *http.Request){
		"/global":                           d.handleGlobalUpload,
		"/volumes/create":                   d.handleCreate,
		"/volumes/copy":                     d.handleCopy,
		"/volumes/request":                  d.handleRequest,
		"/mount":                            d.handleMount,
		"/mount-report":                     d.handleMountReport,
		"/policies/{policy}":                d.handlePolicyUpload,
		"/runtime/{policy}/{volume}":        d.handleRuntimeUpload,
		"/unmount":                          d.handleUnmount,
		"/snapshots/take/{policy}/{volume}": d.handleSnapshotTake,
	}

	if err := addRoute(r, postRouter, "POST", d.Global.Debug); err != nil {
		log.Fatalf("Error starting volmaster: %v", err)
	}

	deleteRouter := map[string]func(http.ResponseWriter, *http.Request){
		"/volumes/remove":      d.handleRemove,
		"/volumes/removeforce": d.handleRemoveForce,
		"/policies/{policy}":   d.handlePolicyDelete,
	}

	if err := addRoute(r, deleteRouter, "DELETE", d.Global.Debug); err != nil {
		log.Fatalf("Error starting volmaster: %v", err)
	}

	getRouter := map[string]func(http.ResponseWriter, *http.Request){
		"/global":                           d.handleGlobal,
		"/policies":                         d.handlePolicyList,
		"/policies/{policy}":                d.handlePolicy,
		"/uses/mounts/{policy}/{volume}":    d.handleUsesMountsVolume,
		"/uses/snapshots/{policy}/{volume}": d.handleUsesMountsSnapshots,
		"/volumes":                          d.handleListAll,
		"/volumes/{policy}":                 d.handleList,
		"/volumes/{policy}/{volume}":        d.handleGet,
		"/runtime/{policy}/{volume}":        d.handleRuntime,
		"/snapshots/{policy}/{volume}":      d.handleSnapshotList,
	}

	if err := addRoute(r, getRouter, "GET", d.Global.Debug); err != nil {
		log.Fatalf("Error starting volmaster: %v", err)
	}

	if d.Global.Debug {
		r.HandleFunc("{action:.*}", d.handleDebug)
	}

	if err := http.ListenAndServe(listen, r); err != nil {
		log.Fatalf("Error starting volmaster: %v", err)
	}
}

func addRoute(r *mux.Router, handlers routeHandlers, method string, debug bool) error {
	for path, f := range handlers {
		if strings.HasSuffix(path, "/") {
			return fmt.Errorf("route path %v has trailing slash", path)
		}
		r.HandleFunc(path, logHandler(path, debug, f)).Methods(method)
		pathSlash := fmt.Sprintf("%v/", path)
		r.HandleFunc(pathSlash, logHandler(pathSlash, debug, f)).Methods(method)
	}
	return nil
}

func logHandler(name string, debug bool, actionFunc func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if debug {
			buf := new(bytes.Buffer)
			io.Copy(buf, r.Body)
			log.Debugf("Dispatching %s with %v", name, strings.TrimSpace(string(buf.Bytes())))
			var writer *io.PipeWriter
			r.Body, writer = io.Pipe()
			go func() {
				io.Copy(writer, buf)
				writer.Close()
			}()
		}

		actionFunc(w, r)
	}
}

func (d *DaemonConfig) handleDebug(w http.ResponseWriter, r *http.Request) {
	io.Copy(os.Stderr, r.Body)
	w.WriteHeader(404)
}

func (d *DaemonConfig) handleGlobalUpload(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, errors.ReadBody.Combine(err))
		return
	}

	global := config.NewGlobalConfig()
	if err := json.Unmarshal(data, global); err != nil {
		httpError(w, errors.UnmarshalGlobal.Combine(err))
		return
	}

	if err := d.Config.PublishGlobal(global); err != nil {
		httpError(w, errors.PublishGlobal.Combine(err))
		return
	}
}

func (d *DaemonConfig) handlePolicyUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyName := vars["policy"]

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, errors.ReadBody.Combine(err))
		return
	}

	policy := config.NewPolicy()
	if err := json.Unmarshal(data, policy); err != nil {
		httpError(w, errors.UnmarshalPolicy.Combine(err))
		return
	}

	if err := d.Config.PublishPolicy(policyName, policy); err != nil {
		httpError(w, errors.PublishPolicy.Combine(err))
		return
	}
}

func (d *DaemonConfig) handlePolicyDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]

	if err := d.Config.DeletePolicy(policy); err != nil {
		httpError(w, errors.PublishGlobal.Combine(err))
		return
	}
}

func (d *DaemonConfig) handlePolicyList(w http.ResponseWriter, r *http.Request) {
	policies, err := d.Config.ListPolicies()
	if err != nil {
		httpError(w, errors.ListPolicy.Combine(err))
		return
	}

	content, err := json.Marshal(policies)
	if err != nil {
		httpError(w, errors.ListPolicy.Combine(err))
		return
	}

	w.Write(content)
}
func (d *DaemonConfig) handlePolicy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]

	policyObj, err := d.Config.GetPolicy(policy)
	if err != nil {
		httpError(w, errors.GetPolicy.Combine(err))
		return
	}

	content, err := json.Marshal(policyObj)
	if err != nil {
		httpError(w, errors.MarshalPolicy.Combine(err))
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleUsesMountsVolume(w http.ResponseWriter, r *http.Request) {
	d.handleUserEndpoints(&config.UseMount{}, w, r)
}

func (d *DaemonConfig) handleUsesMountsSnapshots(w http.ResponseWriter, r *http.Request) {
	d.handleUserEndpoints(&config.UseSnapshot{}, w, r)
}

func (d *DaemonConfig) handleUserEndpoints(ul config.UseLocker, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	volumeName := vars["volume"]

	vc := &config.Volume{
		PolicyName: policy,
		VolumeName: volumeName,
	}

	if err := d.Config.GetUse(ul, vc); err != nil {
		httpError(w, errors.GetMount.Combine(err))
		return
	}

	content, err := json.Marshal(ul)
	if err != nil {
		httpError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleRuntime(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	volumeName := vars["volume"]

	runtime, err := d.Config.GetVolumeRuntime(policy, volumeName)
	if err != nil {
		httpError(w, errors.GetVolume.Combine(err))
		return
	}

	content, err := json.Marshal(runtime)
	if err != nil {
		httpError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleRuntimeUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	volumeName := vars["volume"]

	volume, err := d.Config.GetVolume(policy, volumeName)
	if err != nil {
		httpError(w, errors.GetVolume.Combine(err))
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, errors.ReadBody.Combine(err))
		return
	}

	runtime := config.RuntimeOptions{}
	if err := json.Unmarshal(data, &runtime); err != nil {
		httpError(w, errors.UnmarshalRuntime.Combine(err))
		return
	}

	if err := d.Config.PublishVolumeRuntime(volume, runtime); err != nil {
		httpError(w, errors.PublishRuntime.Combine(err))
		return
	}
}

func (d *DaemonConfig) handleSnapshotList(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	volumeName := vars["volume"]

	volConfig, err := d.Config.GetVolume(policy, volumeName)
	if err != nil {
		httpError(w, errors.GetVolume.Combine(err))
		return
	}

	if volConfig.Backends.Snapshot == "" {
		httpError(w, errors.SnapshotsUnsupported.Combine(errored.Errorf("%q", volConfig)))
		return
	}

	driver, err := backend.NewSnapshotDriver(volConfig.Backends.Snapshot)
	if err != nil {
		httpError(w, errors.GetDriver.Combine(err))
		return
	}

	do := storage.DriverOptions{
		Volume: storage.Volume{
			Name:   volConfig.String(),
			Params: volConfig.DriverOptions,
		},
		Timeout: d.Global.Timeout,
	}

	results, err := driver.ListSnapshots(do)
	if err != nil {
		httpError(w, errors.ListSnapshots.Combine(err))
		return
	}

	content, err := json.Marshal(results)
	if err != nil {
		httpError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleSnapshotTake(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	volume := vars["volume"]

	if err := d.Config.TakeSnapshot(fmt.Sprintf("%v/%v", policy, volume)); err != nil {
		httpError(w, errors.SnapshotFailed.Combine(err))
		return
	}
}

func (d *DaemonConfig) handleCopy(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalRequest(r)
	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	if _, ok := req.Options["snapshot"]; !ok {
		httpError(w, errors.MissingSnapshotOption)
		return
	}

	if _, ok := req.Options["target"]; !ok {
		httpError(w, errors.MissingTargetOption)
		return
	}

	if strings.Contains(req.Options["target"], "/") {
		httpError(w, errors.InvalidVolume.Combine(errored.New("/")))
		return
	}

	volConfig, err := d.Config.GetVolume(req.Policy, req.Volume)
	if err != nil {
		httpError(w, errors.GetVolume.Combine(err))
		return
	}

	if volConfig.Backends.Snapshot == "" {
		httpError(w, errors.SnapshotsUnsupported.Combine(errored.New(volConfig.Backends.Snapshot)))
		return
	}

	driver, err := backend.NewSnapshotDriver(volConfig.Backends.Snapshot)
	if err != nil {
		httpError(w, errors.GetDriver.Combine(err))
		return
	}

	newVolConfig, err := d.Config.GetVolume(req.Policy, req.Volume)
	if err != nil {
		httpError(w, errors.GetVolume.Combine(err))
		return
	}

	newVolConfig.VolumeName = req.Options["target"]

	do := storage.DriverOptions{
		Volume: storage.Volume{
			Name:   volConfig.String(),
			Params: volConfig.DriverOptions,
		},
		Timeout: d.Global.Timeout,
	}

	host, err := os.Hostname()
	if err != nil {
		httpError(w, errors.GetHostname.Combine(err))
		return
	}

	if volConfig.VolumeName == newVolConfig.VolumeName {
		httpError(w, errors.CannotCopyVolume.Combine(errored.Errorf("You cannot copy volume %q onto itself.", volConfig.VolumeName)))
		return
	}

	uc := &config.UseMount{
		Volume:   volConfig.String(),
		Reason:   lock.ReasonCopy,
		Hostname: host,
	}

	snapUC := &config.UseSnapshot{
		Volume: volConfig.String(),
		Reason: lock.ReasonCopy,
	}

	newUC := &config.UseMount{
		Volume:   newVolConfig.String(),
		Reason:   lock.ReasonCopy,
		Hostname: host,
	}

	newSnapUC := &config.UseSnapshot{
		Volume: newVolConfig.String(),
		Reason: lock.ReasonCopy,
	}

	err = lock.NewDriver(d.Config).ExecuteWithMultiUseLock([]config.UseLocker{newUC, newSnapUC, uc, snapUC}, d.Global.Timeout, func(ld *lock.Driver, ucs []config.UseLocker) error {
		if err := d.Config.PublishVolume(newVolConfig); err != nil {
			return err
		}

		if err := driver.CopySnapshot(do, req.Options["snapshot"], newVolConfig.String()); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		httpError(w, errors.PublishVolume.Combine(errored.Errorf(
			"Creating new volume %q from volume %q, snapshot %q",
			req.Options["target"],
			volConfig.String(),
			req.Options["snapshot"],
		)).Combine(err))
		return
	}
}

func (d *DaemonConfig) handleGlobal(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(d.Global)
	if err != nil {
		httpError(w, errors.MarshalGlobal.Combine(err))
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleList(w http.ResponseWriter, r *http.Request) {
	vols, err := d.Config.ListAllVolumes()
	if err != nil {
		httpError(w, errors.ListVolume.Combine(err))
		return
	}

	response := []*config.Volume{}
	for _, vol := range vols {
		parts := strings.SplitN(vol, "/", 2)
		if len(parts) != 2 {
			httpError(w, errors.InvalidVolume.Combine(errored.New(vol)))
			return
		}
		// FIXME make this take a single string and not a split one
		volConfig, err := d.Config.GetVolume(parts[0], parts[1])
		if err != nil {
			httpError(w, errors.ListVolume.Combine(err))
			return
		}

		response = append(response, volConfig)
	}

	content, err := json.Marshal(response)
	if err != nil {
		httpError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleListAll(w http.ResponseWriter, r *http.Request) {
	vols, err := d.Config.ListAllVolumes()
	if err != nil {
		httpError(w, errors.ListVolume.Combine(err))
		return
	}

	response := []*config.Volume{}
	for _, vol := range vols {
		parts := strings.SplitN(vol, "/", 2)
		if len(parts) != 2 {
			httpError(w, errors.InvalidVolume.Combine(errored.New(vol)))
			return
		}
		// FIXME make this take a single string and not a split one
		volConfig, err := d.Config.GetVolume(parts[0], parts[1])
		if err != nil {
			httpError(w, errors.ListVolume.Combine(err))
			return
		}

		response = append(response, volConfig)
	}

	content, err := json.Marshal(response)
	if err != nil {
		httpError(w, errors.ListVolume.Combine(err))
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	volumeName := vars["volume"]

	volConfig, err := d.Config.GetVolume(policy, volumeName)
	if erd, ok := err.(*errored.Error); ok && erd.Contains(errors.NotExists) {
		w.WriteHeader(404)
		return
	} else if err != nil {
		httpError(w, errors.GetVolume.Combine(err))
		return
	}

	content, err := json.Marshal(volConfig)
	if err != nil {
		httpError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleRemove(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalRequest(r)
	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	vc, err := d.Config.GetVolume(req.Policy, req.Volume)
	if err != nil {
		httpError(w, errors.GetVolume.Combine(err))
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		httpError(w, errors.GetHostname.Combine(err))
		return
	}

	uc := &config.UseMount{
		Volume:   vc.String(),
		Reason:   lock.ReasonRemove,
		Hostname: hostname,
	}

	snapUC := &config.UseSnapshot{
		Volume: vc.String(),
		Reason: lock.ReasonRemove,
	}

	etcdRemove := func() error {
		if err := d.Config.RemoveVolume(req.Policy, req.Volume); err != nil {
			return errors.ClearVolume.Combine(errored.New(vc.String())).Combine(err)
		}

		return nil
	}

	complete := func() error {
		if err := d.removeVolume(vc, d.Global.Timeout); err != nil && err != errors.NoActionTaken {
			log.Warn(errors.RemoveImage.Combine(errored.New(vc.String())).Combine(err))
		}

		return etcdRemove()
	}

	err = lock.NewDriver(d.Config).ExecuteWithMultiUseLock([]config.UseLocker{uc, snapUC}, d.Global.Timeout, func(ld *lock.Driver, ucs []config.UseLocker) error {
		exists, err := d.existsVolume(vc)
		if err != nil && err != errors.NoActionTaken {
			return err
		}

		if err == errors.NoActionTaken {
			return complete()
		}

		if !exists {
			etcdRemove()
			return errors.NotExists
		}

		return complete()
	})

	if err == errors.NotExists {
		w.WriteHeader(404)
		return
	}

	if err != nil {
		httpError(w, errors.RemoveVolume.Combine(errored.New(vc.String())).Combine(err))
		return
	}
}

func (d *DaemonConfig) handleRemoveForce(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalRequest(r)
	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	err = d.Config.RemoveVolume(req.Policy, req.Volume)
	if err == errors.NotExists {
		w.WriteHeader(404)
		return
	}

	if err != nil {
		httpError(w, errors.RemoveVolume.Combine(errored.Errorf("%v/%v", req.Policy, req.Volume)).Combine(err))
		return
	}
}

func (d *DaemonConfig) handleUnmount(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalUseMount(r)
	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	req.Reason = lock.ReasonMount
	if err := d.Config.RemoveUse(req, false); err != nil {
		httpError(w, errors.RemoveMount.Combine(err))
		return
	}
}

func (d *DaemonConfig) handleMount(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalUseMount(r)
	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	req.Reason = lock.ReasonMount
	if err := d.Config.PublishUse(req); err != nil {
		httpError(w, errors.PublishMount.Combine(err))
		return
	}
}

func (d *DaemonConfig) handleMountReport(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalUseMount(r)
	if err != nil {
		httpError(w, errors.RefreshMount.Combine(err))
		return
	}

	parts := strings.SplitN(req.Volume, "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		httpError(w, errors.InvalidVolume.Combine(errored.New(req.Volume)))
		return
	}

	_, err = d.Config.GetVolume(parts[0], parts[1])

	if erd, ok := err.(*errored.Error); ok && erd.Contains(errors.NotExists) {
		log.Error("Cannot refresh mount information: volume no longer exists", err)
		w.WriteHeader(404)
		return
	} else if err != nil {
		httpError(w, errors.GetVolume.Combine(err))
		return
	}

	req.Reason = lock.ReasonMount
	if err := d.Config.PublishUseWithTTL(req, d.Global.TTL, client.PrevExist); err != nil {
		httpError(w, errors.RefreshMount.Combine(err))
		return
	}
}

func (d *DaemonConfig) handleRequest(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalRequest(r)
	if err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	tenConfig, err := d.Config.GetVolume(req.Policy, req.Volume)
	if erd, ok := err.(*errored.Error); ok && erd.Contains(errors.NotExists) {
		w.WriteHeader(404)
		return
	} else if err != nil {
		httpError(w, errors.GetVolume.Combine(err))
		return
	}

	content, err := json.Marshal(tenConfig)
	if err != nil {
		httpError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleCreate(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, errors.ReadBody.Combine(err))
		return
	}

	var req config.Request

	if err := json.Unmarshal(content, &req); err != nil {
		httpError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	if req.Policy == "" {
		httpError(w, errors.GetPolicy.Combine(errored.Errorf("policy was blank")))
		return
	}

	if req.Volume == "" {
		httpError(w, errors.GetVolume.Combine(errored.Errorf("volume was blank")))
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		httpError(w, errors.GetHostname.Combine(err))
		return
	}

	policy, err := d.Config.GetPolicy(req.Policy)
	if err != nil {
		httpError(w, errors.GetPolicy.Combine(errored.New(req.Policy).Combine(err)))
		return
	}

	uc := &config.UseMount{
		Volume:   strings.Join([]string{req.Policy, req.Volume}, "/"),
		Reason:   lock.ReasonCreate,
		Hostname: hostname,
	}

	snapUC := &config.UseSnapshot{
		Volume: strings.Join([]string{req.Policy, req.Volume}, "/"),
		Reason: lock.ReasonCreate,
	}

	err = lock.NewDriver(d.Config).ExecuteWithMultiUseLock([]config.UseLocker{uc, snapUC}, d.Global.Timeout, func(ld *lock.Driver, ucs []config.UseLocker) error {
		volConfig, err := d.Config.CreateVolume(req)
		if err != nil {
			return err
		}

		log.Debugf("Volume Create: %#v", *volConfig)

		do, err := d.createVolume(policy, volConfig, d.Global.Timeout)
		if err == errors.NoActionTaken {
			goto publish
		}

		if err != nil {
			return errors.CreateVolume.Combine(err)
		}

		if err := d.formatVolume(volConfig, do); err != nil {
			if err := d.removeVolume(volConfig, d.Global.Timeout); err != nil {
				log.Errorf("Error during cleanup of failed format: %v", err)
			}
			return errors.FormatVolume.Combine(err)
		}

	publish:
		if err := ld.Config.PublishVolume(volConfig); err != nil && err != errors.Exists {
			// FIXME this shouldn't leak down to the client.
			if _, ok := err.(*errored.Error); !ok {
				return errors.PublishVolume.Combine(err)
			}
			return err
		}

		content, err = json.Marshal(volConfig)
		if err != nil {
			return errors.MarshalPolicy.Combine(err)
		}

		w.Write(content)
		return nil
	})

	if err != nil && err != errors.Exists {
		httpError(w, errors.CreateVolume.Combine(err))
		return
	}
}
