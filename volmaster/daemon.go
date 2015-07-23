package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

type request struct {
	Tenant string `json:"tenant"`
	Volume string `json:"volume"`
}

var (
	mutex     = new(sync.Mutex)
	volumeMap = map[string]map[string]struct{}{} // tenant to array of volume names
)

func daemon(config config) {
	r := mux.NewRouter()
	r.HandleFunc("/", config.handleRequest).Methods("POST")

	go scheduleSnapshots(config)

	http.ListenAndServe(":8080", r)

	select {}
}

func (conf config) handleRequest(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "Reading request", err)
		return
	}

	var req request

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
	if _, ok := volumeMap[req.Tenant]; !ok {
		volumeMap[req.Tenant] = map[string]struct{}{}
	}

	volumeMap[req.Tenant][req.Volume] = struct{}{}
	mutex.Unlock()

	content, err = json.Marshal(tenConfig)
	if err != nil {
		httpError(w, "Marshalling response", err)
		return
	}

	w.Write(content)
}
