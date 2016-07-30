package api

import (
	"net/http"

	"github.com/contiv/volplugin/config"
	"github.com/gorilla/mux"
)

// HTTP is a generic interface to HTTP calls. Used by the other interfaces.
type HTTP interface {
	Router(*API) *mux.Router
	HTTPError(http.ResponseWriter, error) // note that it is expected that anything that calls this returns immediately afterwards.
}

// Volplugin is the interface that volplugin needs to provide to the clients
// (docker/k8s/mesos).
type Volplugin interface {
	HTTP
	ReadCreate(*http.Request) (*config.VolumeRequest, error)
	WriteCreate(*config.Volume, http.ResponseWriter) error
	ReadGet(*http.Request) (string, error)
	WriteGet(string, string, http.ResponseWriter) error
	ReadPath(*http.Request) (string, error)
	WritePath(string, http.ResponseWriter) error
	WriteList([]string, http.ResponseWriter) error
	ReadMount(*http.Request) (*Volume, error)
	WriteMount(string, http.ResponseWriter) error
}
