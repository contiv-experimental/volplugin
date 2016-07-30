package apiserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/config"
)

func errExit(ctx *cli.Context, err error) {
	fmt.Printf("\nError: %v\n\n", err)
	cli.ShowAppHelp(ctx)
	os.Exit(1)
}

func unmarshalRequest(r *http.Request) (*config.VolumeRequest, error) {
	cfg := &config.VolumeRequest{}

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return cfg, err
	}

	if err := json.Unmarshal(content, cfg); err != nil {
		return cfg, err
	}

	if cfg.Policy == "" {
		return cfg, errors.New("Policy was blank")
	}

	if cfg.Name == "" {
		return cfg, errors.New("volume was blank")
	}

	return cfg, nil
}

func unmarshalUseMount(r *http.Request) (*config.UseMount, error) {
	cfg := &config.UseMount{}

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return cfg, err
	}

	if err := json.Unmarshal(content, &cfg); err != nil {
		return cfg, err
	}

	if cfg.Volume == "" || cfg.Volume == "/" {
		return cfg, errors.New("volume was blank")
	}

	return cfg, nil
}
