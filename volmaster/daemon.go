package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/librbd"
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

func handleSnapshots(config config) {
	for {
		log.Debug("Running snapshot supervisor")

		for tenant, value := range config {
			mutex.Lock()
			duration, err := time.ParseDuration(config[tenant].Snapshot.Frequency)
			if err != nil {
				panic(fmt.Sprintf("Runtime configuration incorrect; cannot use %q as a snapshot frequency", config[tenant].Snapshot.Frequency))
			}

			if value.UseSnapshots && time.Now().Unix()%int64(duration.Seconds()) == 0 {
				for _, volumes := range volumeMap {
					rbdConfig, err := librbd.ReadConfig("/etc/rbdconfig.json")
					if err != nil {
						log.Errorf("Cannot read RBD configuration: %v", err)
						break
					}
					driver, err := cephdriver.NewCephDriver(rbdConfig, config[tenant].Pool)
					if err != nil {
						log.Errorf("Cannot snap volumes for tenant %q: %v", tenant, err)
						break
					}
					for volume := range volumes {
						now := time.Now()
						log.Infof("Snapping volume \"%s/%s\" at %v", tenant, volume, now)
						if err := driver.NewVolume(volume, config[tenant].Size).CreateSnapshot(now.String()); err != nil {
							log.Errorf("Cannot snap volume %q: %v", volume, err)
						}
					}
				}
			}
			mutex.Unlock()
		}

		time.Sleep(1 * time.Second)
	}
}

func daemon(config config) {
	r := mux.NewRouter()
	r.HandleFunc("/", config.handleRequest).Methods("POST")

	go handleSnapshots(config)

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
