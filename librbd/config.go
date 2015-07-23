package librbd

import (
	"encoding/json"
	"io/ioutil"
)

// RBDConfig provides a JSON representation of some Ceph configuration elements
// that are vital to librbd's use.  librados does not support nested
// configuration; we may sometimes be stuck with this (and in the ansible, are)
// so we need a back up plan on how to manage configuration. This is it. See
// ReadConfig.
type RBDConfig struct {
	MonitorIP string `json:"monitor_ip"`
	UserName  string `json:"username"`
	Secret    string `json:"secret"`
}

// ReadConfig parses a RBDConfig from a JSON encoded file and returns it.
func ReadConfig(path string) (RBDConfig, error) {
	config := RBDConfig{}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(content, &config)

	return config, err
}
