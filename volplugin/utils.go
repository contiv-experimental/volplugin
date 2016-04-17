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

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"
)

var (
	errVolumeResponse = errors.New("Volmaster could not be contacted")
	errVolumeNotFound = errors.New("Volume not found")
)

func (dc *DaemonConfig) mountExists(driver storage.MountDriver, driverOpts storage.DriverOptions) (bool, error) {
	mounts, err := driver.Mounted(dc.Global.Timeout)
	if err != nil {
		return false, err
	}

	mountPath, err := driver.MountPath(driverOpts)
	if err != nil {
		return false, err
	}

	for _, mount := range mounts {
		if mount.Path == mountPath {
			return true, nil
		}
	}

	return false, nil
}

func (dc *DaemonConfig) volumeToDriverOptions(volConfig *config.Volume) (storage.DriverOptions, error) {
	actualSize, err := volConfig.CreateOptions.ActualSize()
	if err != nil {
		return storage.DriverOptions{}, err
	}

	return storage.DriverOptions{
		Volume: storage.Volume{
			Name:   volConfig.String(),
			Size:   actualSize,
			Params: volConfig.DriverOptions,
		},
		FSOptions: storage.FSOptions{
			Type: volConfig.CreateOptions.FileSystem,
		},
		Timeout: dc.Global.Timeout,
	}, nil
}

func (dc *DaemonConfig) structsVolumeName(uc *unmarshalledConfig) (storage.MountDriver, *config.Volume, storage.DriverOptions, error) {
	driverOpts := storage.DriverOptions{}
	volConfig, err := dc.requestVolume(uc.Policy, uc.Name)
	if err != nil {
		return nil, nil, driverOpts, err
	}

	driver, err := backend.NewMountDriver(volConfig.Backends.Mount, dc.Global.MountPath)
	if err != nil {
		return nil, nil, driverOpts, errored.Errorf("loading driver").Combine(err)
	}

	driverOpts, err = dc.volumeToDriverOptions(volConfig)
	if err != nil {
		return nil, nil, driverOpts, errored.Errorf("converting volume to internal structure").Combine(err)
	}

	return driver, volConfig, driverOpts, nil
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

func (dc *DaemonConfig) requestVolume(policy, name string) (*config.Volume, error) {
	var volConfig *config.Volume

	content, err := json.Marshal(config.Request{Volume: name, Policy: policy})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/request", dc.Master), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return nil, errVolumeResponse
	}

	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, errVolumeNotFound
	}

	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errored.Errorf("Could not read response body: %v", err)
	}

	if resp.StatusCode != 200 {
		return nil, errored.Errorf("Status was not 200: was %d: %q", resp.StatusCode, strings.TrimSpace(string(content)))
	}

	if err != nil { // error is from the ReadAll above; we just care more about the status code is all
		return nil, err
	}

	if err := json.Unmarshal(content, &volConfig); err != nil {
		return nil, err
	}

	return volConfig, nil
}

func (dc *DaemonConfig) requestRemove(policy, name string) error {
	content, err := json.Marshal(config.Request{Policy: policy, Volume: name})
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/remove", dc.Master), "application/json", bytes.NewBuffer(content))
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

func (dc *DaemonConfig) requestCreate(policyName, name string, opts map[string]string) error {
	content, err := json.Marshal(config.RequestCreate{Policy: policyName, Volume: name, Opts: opts})
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/create", dc.Master), "application/json", bytes.NewBuffer(content))
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

func splitPath(name string) (string, string, error) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		return "", "", errored.Errorf("Invalid volume name %q", name)
	}

	return parts[0], parts[1], nil
}
