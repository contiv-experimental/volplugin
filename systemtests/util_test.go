package systemtests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

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
	_, err := volcli(fmt.Sprintf("tenant upload %s < /testdata/%s.json", tenantName, fileName))
	return err
}
