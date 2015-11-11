package volmaster

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/config"
)

func errExit(ctx *cli.Context, err error) {
	fmt.Printf("\nError: %v\n\n", err)
	cli.ShowAppHelp(ctx)
	os.Exit(1)
}

func httpError(w http.ResponseWriter, message string, err error) {
	fullError := fmt.Sprintf("%s %v", message, err)

	log.Warnf("Returning HTTP error handling plugin negotiation: %s", fullError)
	http.Error(w, fullError, http.StatusInternalServerError)
}

func unmarshalRequest(r *http.Request) (config.Request, error) {
	var cfg config.Request

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return cfg, err
	}

	if err := json.Unmarshal(content, &cfg); err != nil {
		return cfg, err
	}

	if cfg.Volume == "" {
		return cfg, errors.New("volume was blank")
	}

	return cfg, nil
}

func unmarshalUseConfig(r *http.Request) (*config.UseConfig, error) {
	cfg := &config.UseConfig{}

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return cfg, err
	}

	if err := json.Unmarshal(content, &cfg); err != nil {
		return cfg, err
	}

	if cfg.Volume == nil {
		return cfg, errors.New("volume was blank")
	}

	return cfg, nil
}
