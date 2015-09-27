package systemtests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	utils "github.com/contiv/systemtests-utils"
	"github.com/contiv/volplugin/config"
)

func iterateNodes(fn func(utils.TestbedNode) error) error {
	for _, node := range vagrant.GetNodes() {
		// note that we do all of our work on the monitor instances, so the osd's
		// are left alone
		if strings.HasPrefix(node.GetName(), "mon") {
			if err := fn(node); err != nil {
				return fmt.Errorf(`Error: "%v" on host: %q"`, err, node.GetName())
			}
		}
	}

	return nil
}

func runSSH(cmd string) error {
	return iterateNodes(func(node utils.TestbedNode) error {
		return node.RunCommand(cmd)
	})
}

func mon0cmd(command string) (string, error) {
	return nodeMap["mon0"].RunCommandWithOutput(command)
}

func docker(command string) (string, error) {
	return mon0cmd("docker " + command)
}

func volcli(command string) (string, error) {
	return mon0cmd("volcli " + command)
}

func readIntent(fn string) (*config.TenantConfig, error) {
	content, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	cfg := &config.TenantConfig{}

	if err := json.Unmarshal(content, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func purgeVolume(t *testing.T, host, pool, name string, purgeCeph bool) {
	t.Logf("Purging %s/%s. Purging ceph: %v", host, name, purgeCeph)

	// ignore the error here so we get to the purge if we have to
	nodeMap[host].RunCommand(fmt.Sprintf("docker volume rm %s/%s", pool, name))

	if purgeCeph {
		volcli(fmt.Sprintf("volume remove %s %s", pool, name))
		nodeMap["mon0"].RunCommand(fmt.Sprintf("sudo rbd rm %s/%s", pool, name))
	}
}

func purgeVolumeHost(t *testing.T, pool, host string, purgeCeph bool) {
	purgeVolume(t, host, pool, host, purgeCeph)
}

func createVolumeHost(t *testing.T, pool, host string) {
	createVolume(t, host, pool, host)
}

func createVolume(t *testing.T, host, pool, name string) {
	t.Logf("Creating %s/%s", host, name)

	if out, err := nodeMap[host].RunCommandWithOutput(fmt.Sprintf("docker volume create -d tenant1 --name %s/%s", pool, name)); err != nil {
		t.Log(string(out))
		t.Fatal(err)
	}

	if out, err := nodeMap[host].RunCommandWithOutput(fmt.Sprintf("sudo rbd ls %s | grep -q %s", pool, name)); err != nil {
		t.Log(string(out))
		t.Fatal(err)
	}
}

func rebootstrap() error {
	stopVolplugin()
	stopVolmaster()
	stopEtcd()

	time.Sleep(1 * time.Second)

	if err := startEtcd(); err != nil {
		return err
	}

	if err := startVolmaster(); err != nil {
		return err
	}

	if err := startVolplugin(); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	return nil
}

func uploadIntent(tenantName, fileName string) error {
	log.Infof("Uploading intent %q as tenant %q", fileName, tenantName)
	_, err := volcli(fmt.Sprintf("tenant upload %s < /testdata/%s.json", tenantName, fileName))
	return err
}

func pullUbuntu() error {
	for _, host := range []string{"mon0", "mon1", "mon2"} {
		log.Infof("Pulling ubuntu image on host %q", host)
		if err := nodeMap[host].RunCommand("docker pull ubuntu"); err != nil {
			return err
		}
	}

	return nil
}

func startVolmaster() error {
	log.Infof("Starting the volmaster")
	_, err := nodeMap["mon0"].RunCommandBackground("sudo -E `which volmaster` --debug &>/tmp/volmaster.log &")
	log.Infof("Waiting for volmaster startup")
	time.Sleep(10 * time.Second)
	return err
}

func stopVolmaster() error {
	log.Infof("Stopping the volmaster")
	return nodeMap["mon0"].RunCommand("sudo pkill volmaster")
}

func startVolplugin() error {
	return iterateNodes(volpluginStart)
}

func stopVolplugin() error {
	return iterateNodes(volpluginStop)
}

func volpluginStart(node utils.TestbedNode) error {
	log.Infof("Starting the volplugin on %q", node.GetName())

	// FIXME this is hardcoded because it's simpler. If we move to
	// multimaster or change the monitor subnet, we will have issues.
	_, err := node.RunCommandBackground("sudo -E `which volplugin` --debug --master 192.168.24.10:8080 tenant1 &>/tmp/volplugin.log &")
	return err
}

func volpluginStop(node utils.TestbedNode) error {
	log.Infof("Stopping the volplugin on %q", node.GetName())
	return node.RunCommand("sudo pkill volplugin")
}

func stopEtcd() error {
	log.Infof("Stopping etcd")
	return nodeMap["mon0"].RunCommand("pkill etcd && rm -rf /tmp/etcd")
}

func startEtcd() error {
	log.Infof("Starting etcd")
	_, err := nodeMap["mon0"].RunCommandBackground("etcd -data-dir /tmp/etcd")
	log.Infof("Waiting for etcd to finish starting")
	time.Sleep(1 * time.Second)
	return err
}

func restartDocker() error {
	return iterateNodes(func(node utils.TestbedNode) error {
		log.Infof("Restarting docker on %q", node.GetName())
		return node.RunCommand("sudo service docker restart")
	})
}

func clearContainers() error {
	return iterateNodes(func(node utils.TestbedNode) error {
		log.Infof("Clearing containers on %q", node.GetName())
		return node.RunCommand("docker ps -aq | xargs docker rm -f")
	})
}

func clearVolumes() error {
	return iterateNodes(func(node utils.TestbedNode) error {
		log.Infof("Clearing volumes on %q", node.GetName())
		return node.RunCommand("docker volume ls | tail -n +2 | awk '{ print $2 }' | xargs docker volume rm")
	})
}

func clearRBD() error {
	log.Infof("Clearing rbd images")
	return nodeMap["mon0"].RunCommand("set -e; for img in $(sudo rbd ls); do sudo rbd snap purge $img && sudo rbd rm $img; done")
}
