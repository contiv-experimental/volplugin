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

	for path, f := range postRouter {
		r.HandleFunc(path, logHandler(path, d.Global.Debug, f)).Methods("POST")
	}

	deleteRouter := map[string]func(http.ResponseWriter, *http.Request){
		"/volumes/remove":           d.handleRemove,
		"/volumes/removeforce":      d.handleRemoveForce,
		"/policies/delete/{policy}": d.handlePolicyDelete,
	}

	for path, f := range deleteRouter {
		r.HandleFunc(path, logHandler(path, d.Global.Debug, f)).Methods("DELETE")
	}

	getRouter := map[string]func(http.ResponseWriter, *http.Request){
		"/global":                        d.handleGlobal,
		"/policies":                      d.handlePolicyList,
		"/policies/{policy}":             d.handlePolicy,
		"/uses/mounts/{policy}/{volume}": d.handleUsesMountsVolume,
		"/volumes/":                      d.handleListAll,
		"/volumes/{policy}":              d.handleList,
		"/volumes/{policy}/{volume}":     d.handleGet,
		"/runtime/{policy}/{volume}":     d.handleRuntime,
		"/snapshots/{policy}/{volume}":   d.handleSnapshotList,
	}

	for path, f := range getRouter {
		r.HandleFunc(path, logHandler(path, d.Global.Debug, f)).Methods("GET")
	}

	if d.Global.Debug {
		r.HandleFunc("{action:.*}", d.handleDebug)
	}

	if err := http.ListenAndServe(listen, r); err != nil {
		log.Fatalf("Error starting volmaster: %v", err)
	}
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
		httpError(w, "reading request body", err)
		return
	}

	global := config.NewGlobalConfig()
	if err := json.Unmarshal(data, global); err != nil {
		httpError(w, "Unmarshalling global config upload request", err)
		return
	}

	if err := d.Config.PublishGlobal(global); err != nil {
		httpError(w, "Failed to publish global configuration", err)
		return
	}
}

func (d *DaemonConfig) handlePolicyUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyName := vars["policy"]

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "reading request body", err)
		return
	}

	policy := config.NewPolicy()
	if err := json.Unmarshal(data, policy); err != nil {
		httpError(w, "Unmarshalling policy upload request", err)
		return
	}

	if err := d.Config.PublishPolicy(policyName, policy); err != nil {
		httpError(w, "Failed to upload policy", err)
		return
	}
}

func (d *DaemonConfig) handlePolicyDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]

	if err := d.Config.DeletePolicy(policy); err != nil {
		httpError(w, "Failed to publish global configuration", err)
		return
	}
}

func (d *DaemonConfig) handlePolicyList(w http.ResponseWriter, r *http.Request) {
	policies, err := d.Config.ListPolicies()
	if err != nil {
		httpError(w, "Retrieving list", err)
		return
	}

	content, err := json.Marshal(policies)
	if err != nil {
		httpError(w, "Retrieving list", err)
		return
	}

	w.Write(content)
}
func (d *DaemonConfig) handlePolicy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]

	policyObj, err := d.Config.GetPolicy(policy)
	if err != nil {
		httpError(w, "Retrieving policy", err)
		return
	}

	content, err := json.Marshal(policyObj)
	if err != nil {
		httpError(w, "Marshalling policy response", err)
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleUsesMountsVolume(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	volumeName := vars["volume"]

	vc := &config.Volume{
		PolicyName: policy,
		VolumeName: volumeName,
	}

	mount := &config.UseMount{}

	if err := d.Config.GetUse(mount, vc); err != nil {
		httpError(w, "Getting mount", err)
		return
	}

	content, err := json.Marshal(mount)
	if err != nil {
		httpError(w, "Marshalling response", err)
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
		httpError(w, "Retrieving original volume", err)
		return
	}

	content, err := json.Marshal(runtime)
	if err != nil {
		httpError(w, "Marshalling response", err)
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
		httpError(w, "Retrieving original volume", err)
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "reading request body", err)
		return
	}

	runtime := config.RuntimeOptions{}
	if err := json.Unmarshal(data, &runtime); err != nil {
		httpError(w, "Unmarshalling runtime upload request", err)
		return
	}

	if err := d.Config.PublishVolumeRuntime(volume, runtime); err != nil {
		httpError(w, "Failed publish volume runtime", err)
		return
	}
}

func (d *DaemonConfig) handleSnapshotList(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	volumeName := vars["volume"]

	volConfig, err := d.Config.GetVolume(policy, volumeName)
	if err != nil {
		httpError(w, "Retrieving original volume", err)
		return
	}

	if volConfig.Backends.Snapshot == "" {
		httpError(w, fmt.Sprintf("Backend supporting missing: volume %q cannot support snapshots", volConfig), nil)
		return
	}

	driver, err := backend.NewSnapshotDriver(volConfig.Backends.Snapshot)
	if err != nil {
		httpError(w, "Constructing driver:", err)
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
		httpError(w, "Listing snapshots", err)
		return
	}

	content, err := json.Marshal(results)
	if err != nil {
		httpError(w, "Marshalling results", err)
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleSnapshotTake(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	volume := vars["volume"]

	if err := d.Config.TakeSnapshot(fmt.Sprintf("%v/%v", policy, volume)); err != nil {
		httpError(w, "Failed to take snapshot", err)
		return
	}
}

func (d *DaemonConfig) handleCopy(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalRequest(r)
	if err != nil {
		httpError(w, "Unmarshalling request", err)
		return
	}

	if _, ok := req.Options["snapshot"]; !ok {
		httpError(w, "Could not find snapshot option in request: cannot copy.", nil)
		return
	}

	if _, ok := req.Options["target"]; !ok {
		httpError(w, "Could not find target option in request: cannot copy.", nil)
		return
	}

	if strings.Contains(req.Options["target"], "/") {
		httpError(w, "Volume target contains invalid characters: /", nil)
		return
	}

	volConfig, err := d.Config.GetVolume(req.Policy, req.Volume)
	if err != nil {
		httpError(w, "Retrieving original volume", err)
		return
	}

	if volConfig.Backends.Snapshot == "" {
		httpError(w, fmt.Sprintf("Backend supporting missing: volume %q cannot support snapshots", volConfig), nil)
		return
	}

	driver, err := backend.NewSnapshotDriver(volConfig.Backends.Snapshot)
	if err != nil {
		httpError(w, "Constructing driver:", err)
		return
	}

	newVolConfig, err := d.Config.GetVolume(req.Policy, req.Volume)
	if err != nil {
		httpError(w, "Retrieving original volume", err)
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
		httpError(w, "Retrieving hostname", err)
		return
	}

	if volConfig.VolumeName == newVolConfig.VolumeName {
		httpError(w, fmt.Sprintf("You cannot copy volume %q onto itself.", volConfig.VolumeName), nil)
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
		httpError(w, fmt.Sprintf("Creating new volume %q from volume %q, snapshot %q", req.Options["target"], volConfig.String(), req.Options["snapshot"]), err)
		return
	}
}

func (d *DaemonConfig) handleGlobal(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(d.Global)
	if err != nil {
		httpError(w, "Marshalling global configuration", err)
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleList(w http.ResponseWriter, r *http.Request) {
	vols, err := d.Config.ListAllVolumes()
	if err != nil {
		httpError(w, "Retrieving list", err)
		return
	}

	response := []*config.Volume{}
	for _, vol := range vols {
		parts := strings.SplitN(vol, "/", 2)
		if len(parts) != 2 {
			httpError(w, "Invalid volume name", nil)
			return
		}
		// FIXME make this take a single string and not a split one
		volConfig, err := d.Config.GetVolume(parts[0], parts[1])
		if err != nil {
			httpError(w, "Retrieving list", err)
			return
		}

		response = append(response, volConfig)
	}

	content, err := json.Marshal(response)
	if err != nil {
		httpError(w, "Retrieving list", err)
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleListAll(w http.ResponseWriter, r *http.Request) {
	vols, err := d.Config.ListAllVolumes()
	if err != nil {
		httpError(w, "Retrieving list", err)
		return
	}

	response := []*config.Volume{}
	for _, vol := range vols {
		parts := strings.SplitN(vol, "/", 2)
		if len(parts) != 2 {
			httpError(w, "Invalid volume name", nil)
			return
		}
		// FIXME make this take a single string and not a split one
		volConfig, err := d.Config.GetVolume(parts[0], parts[1])
		if err != nil {
			httpError(w, "Retrieving list", err)
			return
		}

		response = append(response, volConfig)
	}

	content, err := json.Marshal(response)
	if err != nil {
		httpError(w, "Retrieving list", err)
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	volumeName := vars["volume"]

	volConfig, err := d.Config.GetVolume(policy, volumeName)
	if config.NotFound(err) {
		w.WriteHeader(404)
		return
	} else if err != nil {
		httpError(w, "Retrieving volume", err)
		return
	}

	content, err := json.Marshal(volConfig)
	if err != nil {
		httpError(w, "Marshalling response", err)
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleRemove(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalRequest(r)
	if err != nil {
		httpError(w, "unmarshalling request", err)
		return
	}

	vc, err := d.Config.GetVolume(req.Policy, req.Volume)
	if err != nil {
		httpError(w, "obtaining volume configuration", err)
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		httpError(w, "Retrieving hostname", err)
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
			return errored.Errorf("Clearing volume records for %q", vc).Combine(err)
		}

		return nil
	}

	complete := func() error {
		if err := d.removeVolume(vc, d.Global.Timeout); err != nil && err != errNoActionTaken {
			log.Warn(errored.Errorf("Removing image %q", vc).Combine(err))
		}

		return etcdRemove()
	}

	err = lock.NewDriver(d.Config).ExecuteWithMultiUseLock([]config.UseLocker{uc, snapUC}, d.Global.Timeout, func(ld *lock.Driver, ucs []config.UseLocker) error {
		exists, err := d.existsVolume(vc)
		if err != nil && err != errNoActionTaken {
			return err
		}

		if err == errNoActionTaken {
			return complete()
		}

		if !exists {
			etcdRemove()
			return config.ErrNotExist
		}

		return complete()
	})

	if err == config.ErrNotExist {
		w.WriteHeader(404)
		return
	}

	if err != nil {
		httpError(w, fmt.Sprintf("Removing volume %v", vc), err)
		return
	}
}

func (d *DaemonConfig) handleRemoveForce(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalRequest(r)
	if err != nil {
		httpError(w, "unmarshalling request", err)
		return
	}

	err = d.Config.RemoveVolume(req.Policy, req.Volume)
	if err == config.ErrNotExist {
		w.WriteHeader(404)
		return
	}

	if err != nil {
		httpError(w, fmt.Sprintf("Removing volume %v/%v", req.Policy, req.Volume), err)
		return
	}
}

func (d *DaemonConfig) handleUnmount(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalUseMount(r)
	if err != nil {
		httpError(w, "Unmarshalling request", err)
		return
	}

	req.Reason = lock.ReasonMount
	if err := d.Config.RemoveUse(req, false); err != nil {
		httpError(w, "Could not remove mount information", err)
		return
	}
}

func (d *DaemonConfig) handleMount(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalUseMount(r)
	if err != nil {
		httpError(w, "Could not publish mount information", err)
		return
	}

	req.Reason = lock.ReasonMount
	if err := d.Config.PublishUse(req); err != nil {
		httpError(w, "Could not publish mount information", err)
		return
	}
}

func (d *DaemonConfig) handleMountReport(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalUseMount(r)
	if err != nil {
		httpError(w, "Could not refresh mount information", err)
		return
	}

	parts := strings.SplitN(req.Volume, "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		httpError(w, "Invalid volume name", err)
		return
	}

	_, err = d.Config.GetVolume(parts[0], parts[1])

	if config.NotFound(err) {
		log.Error("Cannot refresh mount information: volume no longer exists", err)
		w.WriteHeader(404)
		return
	} else if err != nil {
		httpError(w, "Retrieving volume", err)
		return
	}

	req.Reason = lock.ReasonMount
	if err := d.Config.PublishUseWithTTL(req, d.Global.TTL, client.PrevExist); err != nil {
		httpError(w, "Could not refresh mount information", err)
		return
	}
}

func (d *DaemonConfig) handleRequest(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalRequest(r)
	if err != nil {
		httpError(w, "Unmarshalling request", err)
		return
	}

	tenConfig, err := d.Config.GetVolume(req.Policy, req.Volume)
	if config.NotFound(err) {
		w.WriteHeader(404)
		return
	} else if err != nil {
		httpError(w, "Retrieving Volume", err)
		return
	}

	content, err := json.Marshal(tenConfig)
	if err != nil {
		httpError(w, "Marshalling response", err)
		return
	}

	w.Write(content)
}

func (d *DaemonConfig) handleCreate(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "Reading request", err)
		return
	}

	var req config.RequestCreate

	if err := json.Unmarshal(content, &req); err != nil {
		httpError(w, "Unmarshalling request", err)
		return
	}

	if req.Policy == "" {
		httpError(w, "Reading policy", errored.Errorf("policy was blank"))
		return
	}

	if req.Volume == "" {
		httpError(w, "Reading volume", errored.Errorf("volume was blank"))
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		httpError(w, "Retrieving hostname", err)
		return
	}

	policy, err := d.Config.GetPolicy(req.Policy)
	if err != nil {
		httpError(w, "Retrieving policy", err)
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
		if err == errNoActionTaken {
			goto publish
		}

		if err != nil {
			return errored.Errorf("Creating volume").Combine(err)
		}

		if err := d.formatVolume(volConfig, do); err != nil {
			if err := d.removeVolume(volConfig, d.Global.Timeout); err != nil {
				log.Errorf("Error during cleanup of failed format: %v", err)
			}
			return errored.Errorf("Formatting volume").Combine(err)
		}

	publish:
		if err := ld.Config.PublishVolume(volConfig); err != nil && err != config.ErrExist {
			// FIXME this shouldn't leak down to the client.
			if _, ok := err.(*errored.Error); !ok {
				return errored.Errorf("Error publishing volume").Combine(err)
			}
			return err
		}

		content, err = json.Marshal(volConfig)
		if err != nil {
			return errored.Errorf("Marshalling response").Combine(err)
		}

		w.Write(content)
		return nil
	})

	if err != nil && err != config.ErrExist {
		httpError(w, "Creating volume", err)
		return
	}
}
