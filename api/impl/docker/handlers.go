package docker

import (
	"encoding/json"
	"net/http"

	"github.com/contiv/volplugin/errors"
	"github.com/docker/docker/pkg/plugins"
)

// Activate activates the plugin.
func Activate(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(plugins.Manifest{Implements: []string{"VolumeDriver"}})
	if err != nil {
		NewVolplugin().HTTPError(w, errors.MarshalResponse.Combine(err))
		return
	}

	w.Write(content)
}

// Deactivate deactivates the plugin.
func Deactivate(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

// Capabilities is the API response for docker capabilities requests.
func Capabilities(w http.ResponseWriter, r *http.Request) {
	content, err := json.Marshal(map[string]map[string]string{
		"Capabilities": {
			"Scope": "global",
		},
	})

	if err != nil {
		NewVolplugin().HTTPError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	w.Write(content)
}
