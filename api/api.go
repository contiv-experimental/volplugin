package api

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/contiv/volplugin/api/internals/mount"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"

	log "github.com/Sirupsen/logrus"
)

// Volume abstracts the notion of a volume as it is received from the plugins.
// It is used heavily by the interfaces.
type Volume struct {
	Mountpoint string
	Policy     string
	Name       string
	Options    map[string]string
}

func (v *Volume) String() string {
	return fmt.Sprintf("%s/%s", v.Policy, v.Name)
}

// API is a typed representation of API handlers.
type API struct {
	Volplugin
	Hostname          string
	Client            *config.Client
	Global            **config.Global // double pointer so we can track watch updates
	Lock              *lock.Driver
	lockStopChanMutex sync.Mutex
	lockStopChans     map[string]chan struct{}
	MountCounter      *mount.Counter
	MountCollection   *mount.Collection
}

// NewAPI returns an *API
func NewAPI(volplugin Volplugin, hostname string, client *config.Client, global **config.Global) *API {
	return &API{
		Volplugin:       volplugin,
		Hostname:        hostname,
		Client:          client,
		Global:          global,
		Lock:            lock.NewDriver(client),
		MountCollection: mount.NewCollection(),
		MountCounter:    mount.NewCounter(),
		lockStopChans:   map[string]chan struct{}{},
	}
}

// RESTHTTPError returns a 500 status with the error.
func RESTHTTPError(w http.ResponseWriter, err error) {
	if err == nil {
		err = errors.Unknown
	}

	log.Errorf("Returning HTTP error handling plugin negotiation: %s", err.Error())
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// Action is a catchall for additional driver functions.
func Action(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	log.Debugf("Unknown driver action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Debug("Body content:", string(content))
	w.WriteHeader(503)
}

// LogHandler injects a request logging handler if debugging is active. In either event it will dispatch.
func LogHandler(name string, debug bool, actionFunc func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
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

// GetStorageParameters accepts a Volume API request and turns it into several internal structs.
func (a *API) GetStorageParameters(uc *Volume) (storage.MountDriver, *config.Volume, storage.DriverOptions, error) {
	driverOpts := storage.DriverOptions{}
	volConfig, err := a.Client.GetVolume(uc.Policy, uc.Name)
	if err != nil {
		return nil, nil, driverOpts, err
	}

	driver, err := backend.NewMountDriver(volConfig.Backends.Mount, (*a.Global).MountPath)
	if err != nil {
		return nil, nil, driverOpts, errors.GetDriver.Combine(err)
	}

	driverOpts, err = volConfig.ToDriverOptions((*a.Global).Timeout)
	if err != nil {
		return nil, nil, driverOpts, errors.UnmarshalRequest.Combine(err)
	}

	return driver, volConfig, driverOpts, nil
}

// AddStopChan adds a stop channel for the purposes of controlling mount ttl refresh goroutines
func (a *API) AddStopChan(name string, stopChan chan struct{}) {
	a.lockStopChanMutex.Lock()
	a.lockStopChans[name] = stopChan
	a.lockStopChanMutex.Unlock()
}

// RemoveStopChan removes a stop channel for the purposes of controlling mount ttl refresh goroutines
func (a *API) RemoveStopChan(name string) {
	a.lockStopChanMutex.Lock()
	if test, ok := a.lockStopChans[name]; ok && test != nil {
		a.lockStopChans[name] <- struct{}{}
	}
	delete(a.lockStopChans, name)
	a.lockStopChanMutex.Unlock()
}
