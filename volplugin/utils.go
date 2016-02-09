package volplugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/config"
)

var (
	errVolumeResponse = errors.New("Volmaster could not be contacted")
	errVolumeNotFound = errors.New("Volume not found")
)

func heartbeatMount(master string, ttl time.Duration, payload *config.UseConfig, stop chan struct{}) {
	sleepTime := ttl / 4

	for {
		select {
		case <-stop:
			return
		case <-time.After(sleepTime):
			if err := reportMountStatus(master, payload); err != nil {
				log.Errorf("Could not report mount for host %q to master %q: %v", payload.Hostname, master, err)
				continue
			}
		}
	}
}

func httpError(w http.ResponseWriter, message string, err error) {
	fullError := fmt.Sprintf("%s %v", message, err)

	content, errc := marshalResponse(VolumeResponse{"", fullError})
	if errc != nil {
		log.Warnf("Error received marshalling error response: %v, original error: %s", errc, fullError)
		return
	}

	log.Warnf("Returning HTTP error handling plugin negotiation: %s", fullError)
	http.Error(w, string(content), http.StatusInternalServerError)
}

func unmarshalRequest(body io.Reader) (VolumeRequest, error) {
	vr := VolumeRequest{}

	content, err := ioutil.ReadAll(body)
	if err != nil {
		return vr, err
	}

	err = json.Unmarshal(content, &vr)
	return vr, err
}

func marshalResponse(vr VolumeResponse) ([]byte, error) {
	return json.Marshal(vr)
}

func requestVolumeConfig(host, tenant, name string) (*config.VolumeConfig, error) {
	var volConfig *config.VolumeConfig

	content, err := json.Marshal(config.Request{Volume: name, Tenant: tenant})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/request", host), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return nil, errVolumeResponse
	}

	content, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode == 404 {
		return nil, errVolumeNotFound
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Status was not 200: was %d: %q", resp.StatusCode, strings.TrimSpace(string(content)))
	}

	if err != nil { // error is from the ReadAll above; we just care more about the status code is all
		return nil, err
	}

	if err := json.Unmarshal(content, &volConfig); err != nil {
		return nil, err
	}

	return volConfig, nil
}

func requestRemove(host, tenant, name string) error {
	content, err := json.Marshal(config.Request{Tenant: tenant, Volume: name})
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/remove", host), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}

	content, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("Status was not 200: was %d: %q", resp.StatusCode, strings.TrimSpace(string(content)))
	}

	return nil
}

func requestCreate(host, tenantName, name string, opts map[string]string) error {
	content, err := json.Marshal(config.RequestCreate{Tenant: tenantName, Volume: name, Opts: opts})
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/create", host), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}

	content, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("Status was not 200: was %d: %q", resp.StatusCode, strings.TrimSpace(string(content)))
	}

	return nil
}

func reportMountEndpoint(endpoint, master string, ut *config.UseConfig) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/%s", master, endpoint), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}

	content, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("Status was not 200: was %d: %q", resp.StatusCode, strings.TrimSpace(string(content)))
	}

	return nil
}

func reportMount(master string, ut *config.UseConfig) error {
	return reportMountEndpoint("mount", master, ut)
}

func reportMountStatus(master string, ut *config.UseConfig) error {
	return reportMountEndpoint("mount-report", master, ut)
}

func reportUnmount(master string, ut *config.UseConfig) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/unmount", master), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}

	content, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("Status was not 200: was %d: %q", resp.StatusCode, strings.TrimSpace(string(content)))
	}

	return nil
}

func splitPath(name string) (string, string, error) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Invalid volume name %q", name)
	}

	return parts[0], parts[1], nil
}

func joinPath(tenant, name string) string {
	return strings.Join([]string{tenant, name}, ".")
}

func addStopChan(name string) chan struct{} {
	mountMutex.Lock()
	defer mountMutex.Unlock()

	stopChan := make(chan struct{})

	if sc, ok := mountStopChans[name]; ok {
		sc <- struct{}{}
	}

	mountStopChans[name] = stopChan

	return stopChan
}

func removeStopChan(name string) {
	mountMutex.Lock()
	defer mountMutex.Unlock()

	if sc, ok := mountStopChans[name]; ok {
		sc <- struct{}{}
		delete(mountStopChans, name)
	}
}
