package docker

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/contiv/volplugin/api"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage"
	"github.com/docker/docker/pkg/plugins"
)

// Request is a docker volume request.
type Request struct {
	Request api.VolumeCreateRequest
	Name    string
	Policy  string
}

// WriteResponse writes a docker-compatible volumes api response
func WriteResponse(w http.ResponseWriter, vr *api.VolumeCreateResponse) {
	content, err := json.Marshal(*vr)
	if err != nil {
		api.DockerHTTPError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

// Activate activates the plugin.
func Activate(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(plugins.Manifest{Implements: []string{"VolumeDriver"}})
	if err != nil {
		api.DockerHTTPError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

// Deactivate deactivates the plugin.
func Deactivate(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

// Unmarshal returns a request from the body provided to it.
func Unmarshal(r *http.Request) (*Request, error) {
	defer r.Body.Close()
	vr := api.VolumeCreateRequest{}

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(content, &vr)
	if err != nil {
		return nil, err
	}

	if vr.Name == "" {
		return nil, err
	}

	policy, name, err := storage.SplitName(vr.Name)
	if err != nil {
		return nil, err
	}

	return &Request{
		Request: vr,
		Name:    name,
		Policy:  policy,
	}, nil
}

// Capabilities is the API response for docker capabilities requests.
func Capabilities(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(map[string]map[string]string{
		"Capabilities": {
			"Scope": "global",
		},
	})

	if err != nil {
		api.DockerHTTPError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	w.Write(content)
}
