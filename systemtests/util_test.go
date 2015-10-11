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

func (s *systemtestSuite) iterateNodes(fn func(utils.TestbedNode) error) error {
	wg := sync.WaitGroup{}
	errChan := make(chan error, 3)

	for _, node := range s.vagrant.GetNodes() {
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

func (s *systemtestSuite) runSSH(cmd string) error {
	return s.iterateNodes(func(node utils.TestbedNode) error {
		return node.RunCommand(cmd)
	})
}

func (s *systemtestSuite) mon0cmd(command string) (string, error) {
	return s.nodeMap["mon0"].RunCommandWithOutput(command)
}

func (s *systemtestSuite) docker(command string) (string, error) {
	return s.mon0cmd("docker " + command)
}

func (s *systemtestSuite) volcli(command string) (string, error) {
	return s.mon0cmd("volcli " + command)
}

func (s *systemtestSuite) readIntent(fn string) (*config.TenantConfig, error) {
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

func (s *systemtestSuite) purgeVolume(host, tenant, name string, purgeCeph bool) {
	log.Infof("Purging %s/%s. Purging ceph: %v", host, name, purgeCeph)

	// ignore the error here so we get to the purge if we have to
	s.nodeMap[host].RunCommand(fmt.Sprintf("docker volume rm %s/%s", tenant, name))

	if purgeCeph {
		s.volcli(fmt.Sprintf("volume remove %s %s", tenant, name))
		s.nodeMap["mon0"].RunCommand(fmt.Sprintf("sudo rbd rm rbd/%s", tenant, name))
	}
}

func (s *systemtestSuite) purgeVolumeHost(tenant, host string, purgeCeph bool) {
	s.purgeVolume(host, tenant, host, purgeCeph)
}

func (s *systemtestSuite) createVolumeHost(tenant, host string, opts map[string]string) error {
	return s.createVolume(host, tenant, host, opts)
}

func (s *systemtestSuite) createVolume(host, tenant, name string, opts map[string]string) error {
	log.Infof("Creating %s/%s on %s", tenant, name, host)

	optsStr := []string{}

	if opts != nil {
		for key, value := range opts {
			optsStr = append(optsStr, "--opt")
			optsStr = append(optsStr, key+"="+value)
		}
	}

	cmd := fmt.Sprintf("docker volume create -d volplugin --name %s/%s %s", tenant, name, strings.Join(optsStr, " "))

	if out, err := s.nodeMap[host].RunCommandWithOutput(cmd); err != nil {
		log.Info(string(out))
		return err
	}

	if out, err := s.volcli(fmt.Sprintf("volume get %s %s", tenant, name)); err != nil {
		log.Error(out)
		return err
	}

	if out, err := s.nodeMap[host].RunCommandWithOutput(cmd); err != nil {
		log.Info(string(out))
		return err
	}

	return nil
}

func (s *systemtestSuite) rebootstrap() error {
	s.clearContainers()
	s.clearVolumes()
	s.clearRBD()
	s.stopVolplugin()
	s.stopVolmaster()
	s.stopEtcd()

	if err := s.startEtcd(); err != nil {
		return err
	}

	if err := s.startVolmaster(); err != nil {
		return err
	}

	if err := s.startVolplugin(); err != nil {
		return err
	}

	_, err := s.uploadIntent("tenant1", "intent1")
	if err != nil {
		return err
	}

	return nil
}

func (s *systemtestSuite) uploadIntent(tenantName, fileName string) (string, error) {
	log.Infof("Uploading intent %q as tenant %q", fileName, tenantName)
	return s.volcli(fmt.Sprintf("tenant upload %s < /testdata/%s.json", tenantName, fileName))
}

func (s *systemtestSuite) pullUbuntu() error {
	wg := sync.WaitGroup{}
	errChan := make(chan error, 3)
	for _, host := range []string{"mon0", "mon1", "mon2"} {
		wg.Add(1)
		go func(host string) {
			log.Infof("Pulling ubuntu image on host %q", host)
			if err := s.nodeMap[host].RunCommand("docker pull ubuntu"); err != nil {
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

func (s *systemtestSuite) startVolmaster() error {
	log.Infof("Starting the volmaster")
	_, err := s.nodeMap["mon0"].RunCommandBackground("sudo -E nohup `which volmaster` --debug </dev/null &>/tmp/volmaster.log &")

	log.Infof("Waiting for volmaster startup")
	time.Sleep(10 * time.Millisecond)
	return err
}

func (s *systemtestSuite) stopVolmaster() error {
	log.Infof("Stopping the volmaster")
	return s.nodeMap["mon0"].RunCommand("sudo pkill volmaster")
}

func (s *systemtestSuite) startVolplugin() error {
	// don't sleep if we error, but wait for volplugin to bootstrap if we don't.
	if err := s.iterateNodes(s.volpluginStart); err != nil {
		return err
	}

	return nil
}

func (s *systemtestSuite) stopVolplugin() error {
	return s.iterateNodes(s.volpluginStop)
}

func (s *systemtestSuite) volpluginStart(node utils.TestbedNode) error {
	log.Infof("Starting the volplugin on %q", node.GetName())
	defer time.Sleep(10 * time.Millisecond)

	// FIXME this is hardcoded because it's simpler. If we move to
	// multimaster or change the monitor subnet, we will have issues.
	_, err := node.RunCommandBackground("sudo -E `which volplugin` --debug --master 192.168.24.10:8080 tenant1 &>/tmp/volplugin.log &")
	return err
}

func (s *systemtestSuite) volpluginStop(node utils.TestbedNode) error {
	log.Infof("Stopping the volplugin on %q", node.GetName())
	return node.RunCommand("sudo pkill volplugin")
}

func (s *systemtestSuite) stopEtcd() error {
	log.Infof("Stopping etcd")
	return s.nodeMap["mon0"].RunCommand("pkill etcd && rm -rf /tmp/etcd")
}

func (s *systemtestSuite) startEtcd() error {
	log.Infof("Starting etcd")
	_, err := s.nodeMap["mon0"].RunCommandBackground("nohup etcd -data-dir /tmp/etcd </dev/null &>/dev/null &")
	log.Infof("Waiting for etcd to finish starting")
	time.Sleep(10 * time.Millisecond)
	return err
}

func (s *systemtestSuite) restartDockerHost(node utils.TestbedNode) error {
	log.Infof("Restarting docker on %q", node.GetName())
	// note that for all these restart tasks we error out quietly to avoid other
	// hosts being cleaned up
	node.RunCommand("sudo service docker restart")
	return nil
}

func (s *systemtestSuite) restartDocker() error {
	return s.iterateNodes(s.restartDockerHost)
}

func (s *systemtestSuite) clearContainerHost(node utils.TestbedNode) error {
	log.Infof("Clearing containers on %q", node.GetName())
	node.RunCommand("docker ps -aq | xargs docker rm -f")
	return nil
}

func (s *systemtestSuite) clearContainers() error {
	return s.iterateNodes(s.clearContainerHost)
}

func (s *systemtestSuite) clearVolumeHost(node utils.TestbedNode) error {
	log.Infof("Clearing volumes on %q", node.GetName())
	node.RunCommand("docker volume ls | tail -n +2 | awk '{ print $2 }' | xargs docker volume rm")
	return nil
}

func (s *systemtestSuite) clearVolumes() error {
	return s.iterateNodes(s.clearVolumeHost)
}

func (s *systemtestSuite) clearRBD() error {
	log.Info("Clearing rbd images")
	if out, err := s.nodeMap["mon0"].RunCommandWithOutput("set -e; for img in $(sudo rbd showmapped | tail -n +2 | awk \"{ print \\$5 }\"); do sudo rbd unmap $img; done"); err != nil {
		log.Info(out)
		return err
	}

	return s.nodeMap["mon0"].RunCommand("set -e; for img in $(sudo rbd ls); do sudo rbd snap purge $img && sudo rbd rm $img; done")
}
