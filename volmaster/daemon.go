package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

type configRequest struct {
	Tenant string `json:"tenant"`
	Volume string `json:"volume"`
}

type createRequest struct {
	Tenant string `json:"tenant"`
	Volume string `json:"volume"`
}

var (
	// FIXME this lock is really coarse and dangerous. Split into r/w mutex.
	mutex     = new(sync.Mutex)
	volumeMap = map[string]map[string]createRequest{} // tenant to array of volume names
)

func daemon(config config, debug bool, listen string) {
	r := mux.NewRouter()
	r.HandleFunc("/request", logHandler("/request", debug, config.handleRequest)).Methods("POST")
	r.HandleFunc("/create", logHandler("/create", debug, config.handleCreate)).Methods("POST")

	go scheduleSnapshotPrune(config)
	go scheduleSnapshots(config)

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

func (conf config) handleRequest(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "Reading request", err)
		return
	}

	var req createRequest

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

	mutex.Lock()
	defer mutex.Unlock()

	tenConfig, ok := conf[req.Tenant]
	if ok {
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

func (conf config) handleCreate(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "Reading request", err)
		return
	}

	var req createRequest

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

	log.Infof("Request for tenant %q", req.Tenant)

	tenConfig, ok := conf[req.Tenant]
	if !ok {
		log.Infof("Request for tenant %q cannot be satisfied: not found", req.Tenant)
		httpError(w, "Handling request", fmt.Errorf("Tenant %q not found", req.Tenant))
		return
	}

	mutex.Lock()
	defer mutex.Unlock()
	if _, ok := volumeMap[req.Tenant]; !ok {
		volumeMap[req.Tenant] = map[string]createRequest{}
	}

	if _, ok := volumeMap[req.Tenant][req.Volume]; ok {
		httpError(w, "Handling request", fmt.Errorf("Tenant %q, Image %q could not be used: in use", req.Tenant, req.Volume))
		return
	}

	if err := createImage(conf[req.Tenant], req.Volume, conf[req.Tenant].Size); err != nil {
		volumeMap[req.Tenant][req.Volume] = req
	}

	content, err = json.Marshal(tenConfig)
	if err != nil {
		httpError(w, "Marshalling response", err)
		return
	}

	w.Write(content)
}
