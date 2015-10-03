package systemtests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	utils "github.com/contiv/systemtests-utils"
	"github.com/contiv/volplugin/config"
)

func iterateNodes(fn func(utils.TestbedNode) error) error {
	wg := sync.WaitGroup{}
	errChan := make(chan error, 3)

	for _, node := range vagrant.GetNodes() {
		if strings.HasPrefix(node.GetName(), "mon") {
			wg.Add(1)

			go func(node utils.TestbedNode) {
				if err := fn(node); err != nil {
					errChan <- fmt.Errorf(`Error: "%v" on host: %q"`, err, node.GetName())
				}
				wg.Done()
			}(node)
		}
	}

	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
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

func purgeVolume(host, tenant, name string, purgeCeph bool) {
	log.Infof("Purging %s/%s. Purging ceph: %v", host, name, purgeCeph)

	// ignore the error here so we get to the purge if we have to
	nodeMap[host].RunCommand(fmt.Sprintf("docker volume rm %s/%s", tenant, name))

	if purgeCeph {
		volcli(fmt.Sprintf("volume remove %s %s", tenant, name))
		nodeMap["mon0"].RunCommand(fmt.Sprintf("sudo rbd rm rbd/%s", tenant, name))
	}
}

func purgeVolumeHost(tenant, host string, purgeCeph bool) {
	purgeVolume(host, tenant, host, purgeCeph)
}

func createVolumeHost(tenant, host string, opts map[string]string) error {
	return createVolume(host, tenant, host, opts)
}

func createVolume(host, tenant, name string, opts map[string]string) error {
	log.Infof("Creating %s/%s", host, name)

	optsStr := []string{}

	if opts != nil {
		for key, value := range opts {
			optsStr = append(optsStr, "--opt")
			optsStr = append(optsStr, key+"="+value)
		}
	}

	cmd := fmt.Sprintf("docker volume create -d volplugin --name %s/%s %s", tenant, name, strings.Join(optsStr, " "))

	if out, err := nodeMap[host].RunCommandWithOutput(cmd); err != nil {
		log.Info(string(out))
		return err
	}

	if out, err := volcli(fmt.Sprintf("volume get %s %s", tenant, name)); err != nil {
		log.Error(out)
		return err
	}

	if out, err := nodeMap[host].RunCommandWithOutput(cmd); err != nil {
		log.Info(string(out))
		return err
	}

	return nil
}

func rebootstrap() error {
	clearContainers()
	clearVolumes()
	clearRBD()
	stopVolplugin()
	stopVolmaster()
	stopEtcd()

	time.Sleep(100 * time.Millisecond)

	if err := startEtcd(); err != nil {
		return err
	}

	if err := startVolmaster(); err != nil {
		return err
	}

	if err := startVolplugin(); err != nil {
		return err
	}

	return nil
}

func uploadIntent(tenantName, fileName string) error {
	log.Infof("Uploading intent %q as tenant %q", fileName, tenantName)
	_, err := volcli(fmt.Sprintf("tenant upload %s < /testdata/%s.json", tenantName, fileName))
	return err
}

func pullUbuntu() error {
	wg := sync.WaitGroup{}
	errChan := make(chan error, 3)
	for _, host := range []string{"mon0", "mon1", "mon2"} {
		wg.Add(1)
		go func(host string) {
			log.Infof("Pulling ubuntu image on host %q", host)
			if err := nodeMap[host].RunCommand("docker pull ubuntu"); err != nil {
				errChan <- err
			}
			wg.Done()
		}(host)
	}

	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

func startVolmaster() error {
	log.Infof("Starting the volmaster")
	_, err := nodeMap["mon0"].RunCommandBackground("sudo -E nohup `which volmaster` --debug </dev/null &>/tmp/volmaster.log &")
	log.Infof("Waiting for volmaster startup")
	time.Sleep(100 * time.Millisecond)
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
	_, err := nodeMap["mon0"].RunCommandBackground("nohup etcd -data-dir /tmp/etcd </dev/null &>/dev/null &")
	log.Infof("Waiting for etcd to finish starting")
	time.Sleep(100 * time.Millisecond)
	return err
}

func restartDockerHost(node utils.TestbedNode) error {
	log.Infof("Restarting docker on %q", node.GetName())
	// note that for all these restart tasks we error out quietly to avoid other
	// hosts being cleaned up
	node.RunCommand("sudo service docker restart")
	return nil
}

func restartDocker() error {
	return iterateNodes(restartDockerHost)
}

func clearContainerHost(node utils.TestbedNode) error {
	log.Infof("Clearing containers on %q", node.GetName())
	node.RunCommand("docker ps -aq | xargs docker rm -f")
	return nil
}

func clearContainers() error {
	return iterateNodes(clearContainerHost)
}

func clearVolumeHost(node utils.TestbedNode) error {
	log.Infof("Clearing volumes on %q", node.GetName())
	node.RunCommand("docker volume ls | tail -n +2 | awk '{ print $2 }' | xargs docker volume rm")
	return nil
}

func clearVolumes() error {
	return iterateNodes(clearVolumeHost)
}

func clearRBD() error {
	log.Info("Clearing rbd images")
	if out, err := nodeMap["mon0"].RunCommandWithOutput("set -e; for img in $(sudo rbd showmapped | tail -n +2 | awk \"{ print \\$5 }\"); do sudo rbd unmap $img; done"); err != nil {
		log.Info(out)
		return err
	}

	return nodeMap["mon0"].RunCommand("set -e; for img in $(sudo rbd ls); do sudo rbd snap purge $img && sudo rbd rm $img; done")
}
