// Package client implements a client to the volmaster to acquire locks.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"

	log "github.com/Sirupsen/logrus"
)

var errNotFound = errors.New("not found")

// Driver is the main force behind the lock module, it controls all methods and
// several variables.
type Driver struct {
	master         string
	mountStopChans map[string]chan struct{}
	mountMutex     sync.Mutex
}

// NewDriver constructs a new driver.
func NewDriver(master string) *Driver {
	return &Driver{
		master:         master,
		mountStopChans: map[string]chan struct{}{},
		mountMutex:     sync.Mutex{},
	}
}

// AddStopChan adds a stop channel to a map for heartbeat tracking purposes.
func (d *Driver) AddStopChan(name string) chan struct{} {
	d.mountMutex.Lock()
	defer d.mountMutex.Unlock()

	stopChan := make(chan struct{})

	if sc, ok := d.mountStopChans[name]; ok {
		sc <- struct{}{}
	}

	d.mountStopChans[name] = stopChan

	return stopChan
}

// RemoveStopChan removes a stop channel and mapping from the hearbeat tracker.
func (d *Driver) RemoveStopChan(name string) {
	d.mountMutex.Lock()
	defer d.mountMutex.Unlock()

	if sc, ok := d.mountStopChans[name]; ok {
		sc <- struct{}{}
		delete(d.mountStopChans, name)
	}
}

// HeartbeatMount reports a mount to a volmaster periodically. It loops
// endlessly, and is intended to run as a goroutine. Note the stop channel,
// AddStopChan and RemoveStopChan are used to manage these entities.
func (d *Driver) HeartbeatMount(ttl time.Duration, payload *config.UseMount, stop chan struct{}) {
	sleepTime := ttl / 4

	for {
		select {
		case <-stop:
			return
		case <-time.After(sleepTime):
			log.Debugf("Reporting mount for volume %v", payload.Volume)

			if err := d.ReportMountStatus(payload); err != nil {
				if err == errNotFound {
					break
				}

				log.Errorf("Could not report mount for host %q to master %q: %v", payload.Hostname, d.master, err)
				continue
			}
		}
	}
}

func (d *Driver) reportMountEndpoint(endpoint string, ut *config.UseMount) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/%s", d.master, endpoint), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return errored.Errorf("Could not read response body: %v", err)
	}

	if resp.StatusCode != 200 {
		return errored.Errorf("Status was not 200: was %d: %q", resp.StatusCode, strings.TrimSpace(string(content)))
	}

	if resp.StatusCode == 404 {
		return errNotFound
	}

	return nil
}

// ReportMount reports a new mount to the volmaster.
func (d *Driver) ReportMount(ut *config.UseMount) error {
	return d.reportMountEndpoint("mount", ut)
}

// ReportMountStatus refreshes the mount status (and lock, by axiom).
func (d *Driver) ReportMountStatus(ut *config.UseMount) error {
	return d.reportMountEndpoint("mount-report", ut)
}

// ReportUnmount reports an unmount event to the volmaster, which frees locks.
func (d *Driver) ReportUnmount(ut *config.UseMount) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/unmount", d.master), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return errored.Errorf("Could not read response body: %v", err)
	}

	if resp.StatusCode != 200 {
		return errored.Errorf("Status was not 200: was %d: %q", resp.StatusCode, strings.TrimSpace(string(content)))
	}

	return nil
}
