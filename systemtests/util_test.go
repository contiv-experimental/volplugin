package systemtests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/systemtests-utils"
	"github.com/contiv/vagrantssh"
	"github.com/contiv/volplugin/config"
)

func (s *systemtestSuite) mon0cmd(command string) (string, error) {
	return s.vagrant.GetNode("mon0").RunCommandWithOutput(command)
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
	s.vagrant.GetNode(host).RunCommand(fmt.Sprintf("docker volume rm %s/%s", tenant, name))

	if purgeCeph {
		s.volcli(fmt.Sprintf("volume remove %s %s", tenant, name))
		s.vagrant.GetNode("mon0").RunCommand(fmt.Sprintf("sudo rbd rm rbd/%s.%s", tenant, name))
	}
}

func (s *systemtestSuite) purgeVolumeHost(tenant, host string, purgeCeph bool) {
	s.purgeVolume(host, tenant, host, purgeCeph)
}

func (s *systemtestSuite) createVolumeHost(tenant, host string, opts map[string]string) error {
	return s.createVolume(host, tenant, host, opts)
}

func (s *systemtestSuite) createVolume(host, tenant, name string, opts map[string]string) error {
	log.Infof("Creating %s/%s on %q", tenant, name, host)

	optsStr := []string{}

	if opts != nil {
		for key, value := range opts {
			optsStr = append(optsStr, "--opt")
			optsStr = append(optsStr, key+"="+value)
		}
	}

	cmd := fmt.Sprintf("docker volume create -d volplugin --name %s/%s %s", tenant, name, strings.Join(optsStr, " "))

	if out, err := s.vagrant.GetNode(host).RunCommandWithOutput(cmd); err != nil {
		log.Info(string(out))
		return err
	}

	if out, err := s.volcli(fmt.Sprintf("volume get %s %s", tenant, name)); err != nil {
		log.Error(out)
		return err
	}

	if out, err := s.vagrant.GetNode(host).RunCommandWithOutput(cmd); err != nil {
		log.Info(string(out))
		return err
	}

	return nil
}

func (s *systemtestSuite) rebootstrap() error {
	s.clearContainers()

	stopVolsupervisor(s.vagrant.GetNode("mon0"))
	s.vagrant.IterateNodes(stopVolplugin)
	s.vagrant.IterateNodes(stopVolmaster)
	s.clearRBD()

	utils.ClearEtcd(s.vagrant.GetNode("mon0"))

	if err := s.restartDocker(); err != nil {
		return err
	}

	if err := s.vagrant.IterateNodes(startVolmaster); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	if err := startVolsupervisor(s.vagrant.GetNode("mon0")); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	if err := s.vagrant.IterateNodes(startVolplugin); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

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

func (s *systemtestSuite) pullDebian() error {
	wg := sync.WaitGroup{}
	errChan := make(chan error, 3)
	for _, host := range s.vagrant.GetNodes() {
		wg.Add(1)
		go func(node vagrantssh.TestbedNode) {
			log.Infof("Pulling debian image on host %q", node.GetName())
			if err := node.RunCommand("docker pull debian"); err != nil {
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

func startVolsupervisor(node vagrantssh.TestbedNode) error {
	log.Infof("Starting the volsupervisor on %q", node.GetName())
	return node.RunCommandBackground("sudo -E nohup `which volsupervisor` --debug </dev/null &>/tmp/volsupervisor.log &")
}

func stopVolsupervisor(node vagrantssh.TestbedNode) error {
	log.Infof("Stopping the volsupervisor on %q", node.GetName())
	return node.RunCommand("sudo pkill volsupervisor")
}

func startVolmaster(node vagrantssh.TestbedNode) error {
	log.Infof("Starting the volmaster on %q", node.GetName())
	err := node.RunCommandBackground("sudo -E nohup `which volmaster` --debug --ttl 5 </dev/null &>/tmp/volmaster.log &")
	log.Infof("Waiting for volmaster startup on %q", node.GetName())
	time.Sleep(10 * time.Millisecond)
	return err
}

func stopVolmaster(node vagrantssh.TestbedNode) error {
	log.Infof("Stopping the volmaster on %q", node.GetName())
	return node.RunCommand("sudo pkill volmaster")
}

func startVolplugin(node vagrantssh.TestbedNode) error {
	log.Infof("Starting the volplugin on %q", node.GetName())
	defer time.Sleep(10 * time.Millisecond)

	// FIXME this is hardcoded because it's simpler. If we move to
	// multimaster or change the monitor subnet, we will have issues.
	return node.RunCommandBackground("sudo -E `which volplugin` --debug --ttl 5 &>/tmp/volplugin.log &")
}

func stopVolplugin(node vagrantssh.TestbedNode) error {
	log.Infof("Stopping the volplugin on %q", node.GetName())
	return node.RunCommand("sudo pkill volplugin")
}

func restartDockerHost(node vagrantssh.TestbedNode) error {
	log.Infof("Restarting docker on %q", node.GetName())
	// note that for all these restart tasks we error out quietly to avoid other
	// hosts being cleaned up
	node.RunCommand("sudo service docker restart")
	return nil
}

func (s *systemtestSuite) restartDocker() error {
	return s.vagrant.IterateNodes(restartDockerHost)
}

func (s *systemtestSuite) clearContainerHost(node vagrantssh.TestbedNode) error {
	log.Infof("Clearing containers on %q", node.GetName())
	node.RunCommand("docker ps -aq | xargs docker rm -f")
	return nil
}

func (s *systemtestSuite) clearContainers() error {
	return s.vagrant.IterateNodes(s.clearContainerHost)
}

func (s *systemtestSuite) clearVolumeHost(node vagrantssh.TestbedNode) error {
	log.Infof("Clearing volumes on %q", node.GetName())
	node.RunCommand("docker volume ls | tail -n +2 | awk '{ print $2 }' | xargs docker volume rm")
	return nil
}

func (s *systemtestSuite) clearVolumes() error {
	return s.vagrant.IterateNodes(s.clearVolumeHost)
}

func (s *systemtestSuite) clearRBD() error {
	log.Info("Clearing rbd images")
	if out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("set -e; for img in $(sudo rbd showmapped | tail -n +2 | awk \"{ print \\$5 }\"); do sudo umount $img; sudo rbd unmap $img; done"); err != nil {
		log.Info(out)
		return err
	}

	return s.vagrant.GetNode("mon0").RunCommand("set -e; for img in $(sudo rbd ls); do sudo rbd snap purge $img && sudo rbd rm $img; done")
}
