package volplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/config"
)

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

func requestTenantConfig(host, pool, name string) (*config.VolumeConfig, error) {
	var volConfig *config.VolumeConfig

	content, err := json.Marshal(config.Request{Volume: name, Pool: pool})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/request", host), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}

	content, err = ioutil.ReadAll(resp.Body)

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

func requestCreate(host, tenantName, pool, name string, opts map[string]string) error {
	content, err := json.Marshal(config.RequestCreate{Tenant: tenantName, Volume: name, Pool: pool, Opts: opts})
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

func reportMount(host string, mt *config.MountConfig) error {
	content, err := json.Marshal(mt)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/mount", host), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}

	content, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("Status was not 200: was %d: %q", resp.StatusCode, strings.TrimSpace(string(content)))
	}

	return nil
}

func reportUnmount(host string, mt *config.MountConfig) error {
	content, err := json.Marshal(mt)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/unmount", host), "application/json", bytes.NewBuffer(content))
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
