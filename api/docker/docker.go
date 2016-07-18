package docker

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/api"
	"github.com/contiv/volplugin/errors"
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

	policy, name, err := splitPath(vr.Name)
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

func splitPath(name string) (string, string, error) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.InvalidVolume.Combine(errored.New(name))
	}

	return parts[0], parts[1], nil
}
