package systemtests

import (
	"os"
	"strings"
	"testing"

	log "github.com/Sirupsen/logrus"
	utils "github.com/contiv/systemtests-utils"
)

var (
	vagrant = utils.Vagrant{}
	nodeMap = map[string]utils.TestbedNode{}
)

func setNodeMap() {
	for _, node := range vagrant.GetNodes() {
		nodeMap[node.GetName()] = node
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("HOST_TEST") != "" {
		os.Exit(0)
	}

	log.Infof("Bootstrapping system tests")

	if err := vagrant.Setup(false, "", 6); err != nil {
		log.Fatalf("Vagrant is not working or nodes are not available: %v", err)
	}

	setNodeMap()

	if err := restartDocker(); err != nil {
		log.Fatalf("Could not restart docker")
	}

	if err := clearContainers(); err != nil && !strings.Contains(err.Error(), "Process exited with: 123") {
		log.Fatalf("Could not clean up containers: %#v", err)
	}

	if err := clearVolumes(); err != nil && !strings.Contains(err.Error(), "Process exited with: 1") {
		log.Fatalf("Could not clear volumes for docker")
	}

	if err := pullUbuntu(); err != nil {
		log.Fatalf("Could not pull necessary ubuntu docker image")
	}

	// rebootstrap to avoid leaving state around from cleanups
	if err := rebootstrap(); err != nil {
		log.Fatalf("Could not rebootstrap: %v", err)
	}

	if err := uploadIntent("tenant1", "intent1"); err != nil {
		log.Fatalf("Could not upload tenant1 intent: %v", err)
	}

	exitCode := m.Run()

	log.Infof("Tearing down system test facilities")

	clearContainers()
	clearVolumes()
	restartDocker()

	if err := stopVolplugin(); err != nil {
		log.Errorf("Volplugin could not be stopped: %v", err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	if err := stopVolmaster(); err != nil {
		log.Errorf("Volmaster could not be stopped: %v", err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	if err := stopEtcd(); err != nil {
		log.Errorf("etcd could not be stopped: %v", err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}
