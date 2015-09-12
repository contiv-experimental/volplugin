package volplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

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

func requestTenantConfig(host, tenantName, volumeName string) (*config.TenantConfig, error) {
	var tenConfig *config.TenantConfig

	content, err := json.Marshal(config.Request{tenantName, volumeName})
	if err != nil {
		return tenConfig, err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/request", host), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return tenConfig, err
	}

	if resp.StatusCode != 200 {
		return tenConfig, fmt.Errorf("Status was not 200: was %d", resp.StatusCode)
	}

	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return tenConfig, err
	}

	if err := json.Unmarshal(content, &tenConfig); err != nil {
		return tenConfig, err
	}

	return tenConfig, nil
}

func requestCreate(host, tenantName, volumeName string) error {
	content, err := json.Marshal(config.Request{tenantName, volumeName})
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/create", host), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("Status was not 200: was %d", resp.StatusCode)
	}

	return nil
}
