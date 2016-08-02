package docker

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/contiv/volplugin/api"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage"
	"github.com/gorilla/mux"

	log "github.com/Sirupsen/logrus"
)

// Volplugin implements the docker volumes API via the interfaces in api/interfaces.go.
type Volplugin struct{}

// NewVolplugin initializes the docker api interface for volplugin.
func NewVolplugin() api.Volplugin {
	return &Volplugin{}
}

// Router returns a docker-compatible HTTP gorilla/mux router. If the debug
// global is set, handlers will be wrapped in a request logger.
func (v *Volplugin) Router(a *api.API) *mux.Router {
	var routeMap = map[string]func(http.ResponseWriter, *http.Request){
		"/Plugin.Activate":           Activate,
		"/Plugin.Deactivate":         Deactivate,
		"/VolumeDriver.Capabilities": Capabilities,
		"/VolumeDriver.Create":       a.Create,
		"/VolumeDriver.Remove":       a.Path, // we never actually remove through docker's interface.
		"/VolumeDriver.Path":         a.Path,
		"/VolumeDriver.Get":          a.Get,
		"/VolumeDriver.List":         a.List,
		"/VolumeDriver.Mount":        a.Mount,
		"/VolumeDriver.Unmount":      a.Unmount,
	}

	router := mux.NewRouter()
	s := router.Methods("POST").Subrouter()

	for key, value := range routeMap {
		parts := strings.SplitN(key, ".", 2)
		s.HandleFunc(key, api.LogHandler(parts[1], (*a.Global).Debug, value))
	}

	if (*a.Global).Debug {
		s.HandleFunc("{action:.*}", api.Action)
	}

	return router
}

// HTTPError returns a 200 status to docker with an error struct. It returns
// 500 if marshalling failed.
func (v *Volplugin) HTTPError(w http.ResponseWriter, err error) {
	content, errc := json.Marshal(Response{Err: err.Error()})
	if errc != nil {
		http.Error(w, errc.Error(), http.StatusInternalServerError)
		return
	}

	log.Errorf("Returning HTTP error handling plugin negotiation: %s", err.Error())
	http.Error(w, string(content), http.StatusOK)
}

// ReadCreate reads a create request from docker and parses it into a policy/volume.
func (v *Volplugin) ReadCreate(r *http.Request) (*config.VolumeRequest, error) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, errors.ReadBody.Combine(err)
	}

	var req VolumeCreateRequest

	if err := json.Unmarshal(content, &req); err != nil {
		return nil, errors.UnmarshalRequest.Combine(err)
	}

	policy, volume, err := storage.SplitName(req.Name)
	if err != nil {
		return nil, errors.UnmarshalRequest.Combine(errors.InvalidVolume).Combine(err)
	}

	return &config.VolumeRequest{
		Policy:  policy,
		Name:    volume,
		Options: req.Opts,
	}, nil
}

// WriteCreate writes the response to a create request back to docker.
func (v *Volplugin) WriteCreate(volConfig *config.Volume, w http.ResponseWriter) error {
	content, err := json.Marshal(Response{})
	if err != nil {
		return err
	}

	_, err = w.Write(content)
	return err
}

// ReadGet reads requests for the various Get endpoints (which all do the same thing)
func (v *Volplugin) ReadGet(r *http.Request) (string, error) {
	vc := &VolumeGetRequest{}

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(content, vc); err != nil {
		return "", err
	}

	_, _, err = storage.SplitName(vc.Name)
	if err != nil {
		return "", err
	}

	return vc.Name, nil
}

// WriteGet writes an appropriate response to Get calls.
func (v *Volplugin) WriteGet(name, mountpoint string, w http.ResponseWriter) error {
	content, err := json.Marshal(VolumeGetResponse{Volume: Volume{Name: name, Mountpoint: mountpoint}})
	if err != nil {
		return err
	}

	_, err = w.Write(content)
	return err
}

func unmarshal(r *http.Request) (*VolumeCreateRequest, error) {
	v := &VolumeCreateRequest{}

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	r.Body.Close()
	if err := json.Unmarshal(content, v); err != nil {
		return nil, err
	}

	return v, nil
}

// ReadPath reads a path request.
func (v *Volplugin) ReadPath(r *http.Request) (string, error) {
	vol, err := unmarshal(r)
	if err != nil {
		return "", err
	}

	return vol.Name, nil
}

// WritePath writes an appropriate response to Path calls.
func (v *Volplugin) WritePath(mountpoint string, w http.ResponseWriter) error {
	content, err := json.Marshal(Response{Mountpoint: mountpoint})
	if err != nil {
		return err
	}

	_, err = w.Write(content)
	return err
}

// WriteList writes out a list of volume names to the requesting docker.
func (v *Volplugin) WriteList(volumes []string, w http.ResponseWriter) error {
	response := VolumeList{}
	for _, volume := range volumes {
		response.Volumes = append(response.Volumes, Volume{Name: volume})
	}

	content, err := json.Marshal(response)
	if err != nil {
		return err
	}

	_, err = w.Write(content)
	return err
}

// ReadMount reads a mount request and returns the name of the volume to mount.
//
// NOTE: this is the same for both path and mount and unmount. The docker
// implementation provides us with no other information.
func (v *Volplugin) ReadMount(r *http.Request) (*api.Volume, error) {
	vol, err := unmarshal(r)
	if err != nil {
		return nil, err
	}

	policy, name, err := storage.SplitName(vol.Name)
	if err != nil {
		return nil, err
	}

	return &api.Volume{Policy: policy, Name: name}, nil
}

// WriteMount writes the mountpoint as a reply to a mount request.
func (v *Volplugin) WriteMount(mountPoint string, w http.ResponseWriter) error {
	content, err := json.Marshal(Response{Mountpoint: mountPoint})
	if err != nil {
		return err
	}

	_, err = w.Write(content)
	return err
}
