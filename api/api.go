package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage/control"

	log "github.com/Sirupsen/logrus"
)

// VolumeRequest is taken from
// https://github.com/calavera/docker-volume-api/blob/master/api.go#L23
type VolumeRequest struct {
	Name string
	Opts map[string]string
}

// VolumeResponse is taken from
// https://github.com/calavera/docker-volume-api/blob/master/api.go#L23
type VolumeResponse struct {
	Mountpoint string
	Err        string
}

// API is a typed representation of API handlers.
type API struct {
	DockerPlugin bool
	Client       *config.Client
	Global       **config.Global // double pointer so we can track watch updates
}

// NewAPI returns an *API
func NewAPI(client *config.Client, global **config.Global, dockerPlugin bool) *API {
	return &API{Client: client, Global: global, DockerPlugin: dockerPlugin}
}

// Create fully creates a volume
func (a *API) Create(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		a.HTTPError(w, errors.ReadBody.Combine(err))
		return
	}

	var req VolumeRequest

	if err := json.Unmarshal(content, &req); err != nil {
		a.HTTPError(w, errors.UnmarshalRequest.Combine(err))
		return
	}

	parts := strings.SplitN(req.Name, "/", 2)
	if len(parts) != 2 {
		a.HTTPError(w, errors.UnmarshalRequest.Combine(errors.InvalidVolume))
		return
	}

	policy, volume := parts[0], parts[1]

	if policy == "" {
		a.HTTPError(w, errors.GetPolicy.Combine(errored.Errorf("policy was blank for volume %q", req.Name)))
		return
	}

	if volume == "" {
		a.HTTPError(w, errors.GetVolume.Combine(errored.Errorf("volume was blank for volume %q", req.Name)))
		return
	}

	if vol, err := a.Client.GetVolume(policy, volume); err == nil && vol != nil {
		a.HTTPError(w, err)
		return
	}

	log.Infof("Creating volume %q", req.Name)

	hostname, err := os.Hostname()
	if err != nil {
		a.HTTPError(w, errors.GetHostname.Combine(err))
		return
	}

	policyObj, err := a.Client.GetPolicy(policy)
	if err != nil {
		a.HTTPError(w, errors.GetPolicy.Combine(errored.New(policy).Combine(err)))
		return
	}

	uc := &config.UseMount{
		Volume:   strings.Join([]string{policy, volume}, "/"),
		Reason:   lock.ReasonCreate,
		Hostname: hostname,
	}

	snapUC := &config.UseSnapshot{
		Volume: strings.Join([]string{policy, volume}, "/"),
		Reason: lock.ReasonCreate,
	}

	global := *a.Global

	err = lock.NewDriver(a.Client).ExecuteWithMultiUseLock([]config.UseLocker{uc, snapUC}, global.Timeout, func(ld *lock.Driver, ucs []config.UseLocker) error {
		volConfig, err := a.Client.CreateVolume(config.Request{Policy: policy, Volume: volume, Options: req.Opts})
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
		a.HTTPError(w, errors.CreateVolume.Combine(err))
		return
	}
}

// HTTPError is a generic HTTP error function that works across the plugin and
// REST interfaces. It is intended to be used by handlers that exist in this
// package.
func (a *API) HTTPError(w http.ResponseWriter, err error) {
	if a.DockerPlugin {
		DockerHTTPError(w, err)
	} else {
		RESTHTTPError(w, err)
	}
}

// DockerHTTPError returns a 200 status to docker with an error struct. It returns
// 500 if marshalling failed.
func DockerHTTPError(w http.ResponseWriter, err error) {
	content, errc := json.Marshal(VolumeResponse{"", err.Error()})
	if errc != nil {
		http.Error(w, errc.Error(), http.StatusInternalServerError)
		return
	}

	log.Warnf("Returning HTTP error handling plugin negotiation: %s", err.Error())
	http.Error(w, string(content), http.StatusOK)
}

// RESTHTTPError returns a 500 status with the error.
func RESTHTTPError(w http.ResponseWriter, err error) {
	log.Warnf("Returning HTTP error handling plugin negotiation: %s", err.Error())
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
