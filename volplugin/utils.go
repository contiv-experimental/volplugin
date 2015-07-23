package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
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

func requestTenantConfig(tenantName, volumeName string) (configTenant, error) {
	var tenConfig configTenant

	content, err := json.Marshal(request{tenantName, volumeName})
	if err != nil {
		return tenConfig, err
	}

	resp, err := http.Post("http://localhost:8080", "application/json", bytes.NewBuffer(content))
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
