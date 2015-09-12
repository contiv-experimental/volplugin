package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/config"
	"github.com/gorilla/mux"
)

type daemonConfig struct {
	config *config.TopLevelConfig
}

func daemon(config *config.TopLevelConfig, debug bool, listen string) {
	d := daemonConfig{config}
	r := mux.NewRouter()
	r.HandleFunc("/request", logHandler("/request", debug, d.handleRequest)).Methods("POST")
	r.HandleFunc("/create", logHandler("/create", debug, d.handleCreate)).Methods("POST")

	go scheduleSnapshotPrune(d.config)
	go scheduleSnapshots(d.config)

	http.ListenAndServe(listen, r)

	select {}
}

func logHandler(name string, debug bool, actionFunc func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
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

func (d daemonConfig) handleMount(w http.ResponseWriter, r *http.Request) {
	_, err := unmarshalRequest(r)
	if err != nil {
		httpError(w, "Unmarshalling request", err)
	}
}

func (d daemonConfig) handleRequest(w http.ResponseWriter, r *http.Request) {
	req, err := unmarshalRequest(r)
	if err != nil {
		httpError(w, "Unmarshalling request", err)
		return
	}

	tenConfig, err := d.config.GetVolume(req.Volume)
	if err == nil {
		content, err := json.Marshal(tenConfig)
		if err != nil {
			httpError(w, "Marshalling response", err)
			return
		}

		w.Write(content)
		return
	}

	w.WriteHeader(404)
}

func (d daemonConfig) handleCreate(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "Reading request", err)
		return
	}

	var req config.RequestCreate

	if err := json.Unmarshal(content, &req); err != nil {
		httpError(w, "Unmarshalling request", err)
		return
	}

	if req.Tenant == "" {
		httpError(w, "Reading tenant", errors.New("tenant was blank"))
		return
	}

	if req.Volume == "" {
		httpError(w, "Reading tenant", errors.New("volume was blank"))
		return
	}

	tenConfig, err := d.config.CreateVolume(req.Volume, req.Tenant)
	if err != config.ErrExist {
		if err := createImage(tenConfig, req.Volume); err != nil {
			httpError(w, "Creating volume", err)
			return
		}
	} else if err != nil && err != config.ErrExist {
		httpError(w, "Creating volume", err)
		return
	}

	content, err = json.Marshal(tenConfig)
	if err != nil {
		httpError(w, "Marshalling response", err)
		return
	}

	w.Write(content)
}
