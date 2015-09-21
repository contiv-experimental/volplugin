package systemtests

import (
	"os"
	"strings"
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
	if os.Getenv("HOST_TEST") != "" {
		os.Exit(0)
	}

	log.Infof("Bootstrapping system tests")

	if err := vagrant.Setup(false, "", 6); err != nil {
		log.Fatalf("Vagrant is not working or nodes are not available: %v", err)
	}

	setNodeMap()

	if err := rebootstrap(); err != nil {
		log.Fatalf("Could not bootstrap cluster: %v", err)
	}

	if err := uploadIntent("tenant1", "intent1"); err != nil {
		log.Fatalf("Intent could not be uploaded: %v", err)
	}

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

func setNodeMap() {
	for _, node := range vagrant.GetNodes() {
		nodeMap[node.GetName()] = node
	}
}

func pullUbuntu() error {
	for _, host := range []string{"mon0", "mon1", "mon2"} {
		if err := nodeMap[host].RunCommand("docker pull ubuntu"); err != nil {
			return err
		}
	}

	return nil
}

func startVolmaster() error {
	_, err := nodeMap["mon0"].RunCommandBackground("sudo -E `which volmaster` --debug &>/tmp/volmaster.log &")
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
	_, err := node.RunCommandBackground("sudo -E `which volplugin` --debug --master 192.168.24.10:8080 tenant1 &>/tmp/volplugin.log &")
	return err
}

func volpluginStop(node utils.TestbedNode) error {
	return node.RunCommand("sudo pkill volplugin")
}

func stopEtcd() error {
	return nodeMap["mon0"].RunCommand("pkill etcd && rm -rf /tmp/etcd")
}

func startEtcd() error {
	_, err := nodeMap["mon0"].RunCommandBackground("etcd -data-dir /tmp/etcd")
	time.Sleep(1 * time.Second)
	return err
}

func restartDocker() error {
	return iterateNodes(func(node utils.TestbedNode) error {
		return node.RunCommand("sudo service docker restart")
	})
}

func clearContainers() error {
	return iterateNodes(func(node utils.TestbedNode) error {
		return node.RunCommand("docker ps -aq | xargs docker rm -f")
	})
}

func clearVolumes() error {
	return iterateNodes(func(node utils.TestbedNode) error {
		return node.RunCommand("docker volume ls | tail -n +2 | awk '{ print $2 }' | xargs docker volume rm")
	})
}

func clearRBD() error {
	return nodeMap["mon0"].RunCommand("set -e; for img in $(sudo rbd ls); do sudo rbd snap purge $img && sudo rbd rm $img; done")
}
