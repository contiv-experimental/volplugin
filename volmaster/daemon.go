package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

type request struct {
	tenant string
}

func daemon(config config) {
	r := mux.NewRouter()
	r.HandleFunc("/", config.handleRequest).Methods("POST")

	http.ListenAndServe(":8000", r)

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

	if req.tenant == "" {
		httpError(w, "Reading tenant", errors.New("tenant was blank"))
		return
	}

	log.Infof("Request for tenant %q", req.tenant)

	tenConfig, ok := conf[req.tenant]
	if !ok {
		log.Infof("Request for tenant %q cannot be satisfied: not found", req.tenant)
		httpError(w, "Handling request", fmt.Errorf("Tenant %q not found", req.tenant))
		return
	}

	content, err = json.Marshal(tenConfig)
	if err != nil {
		httpError(w, "Marshalling response", err)
		return
	}

	w.Write(content)
}
