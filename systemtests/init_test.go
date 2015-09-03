package systemtests

import (
	"os"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	utils "github.com/contiv/systemtests-utils"
)

var (
	vagrant = utils.Vagrant{}
	nodeMap = map[string]utils.TestbedNode{}
)

func TestMain(m *testing.M) {
	if err := vagrant.Setup(false, "", 6); err != nil {
		log.Fatalf("Vagrant is not working or nodes are not available: %v", err)
	}

	setNodeMap()

	if err := startVolmaster(); err != nil {
		log.Fatalf("Volmaster could not be started: %v", err)
	}

	if err := startVolplugin(); err != nil {
		log.Fatalf("Volplugin could not be started: %v", err)
	}

	exitCode := m.Run()

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

	os.Exit(exitCode)
}

func setNodeMap() {
	for _, node := range vagrant.GetNodes() {
		nodeMap[node.GetName()] = node
	}
}

func startVolmaster() error {
	_, err := nodeMap["mon0"].RunCommandBackground("sudo -E `which volmaster` /etc/volmaster.json &>/tmp/volmaster.log &")
	time.Sleep(10 * time.Second)
	return err
}

func stopVolmaster() error {
	return nodeMap["mon0"].RunCommand("sudo pkill volmaster")
}

func startVolplugin() error {
	return iterateNodes(volpluginStart)
}

func stopVolplugin() error {
	return iterateNodes(volpluginStop)
}

func volpluginStart(node utils.TestbedNode) error {
	// FIXME this is hardcoded because it's simpler. If we move to
	// multimaster or change the monitor subnet, we will have issues.
	_, err := node.RunCommandBackground("sudo -E `which volplugin` --master 192.168.24.10:8080 tenant1 &>/tmp/volplugin.log &")
	return err
}

func volpluginStop(node utils.TestbedNode) error {
	return node.RunCommand("sudo pkill volplugin")
}
