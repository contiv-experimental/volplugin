package systemtests

import (
	"fmt"
	"strings"
	"testing"

	utils "github.com/contiv/systemtests-utils"
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

func purgeVolume(t *testing.T, host, name string, purgeCeph bool) {
	t.Logf("Purging %s/%s. Purging ceph: %v", host, name, purgeCeph)

	// ignore the error here so we get to the purge if we have to
	nodeMap[host].RunCommand("docker volume rm rbd/" + name)
	nodeMap[host].RunCommand("volcli volume remove rbd " + name)

	if purgeCeph {
		nodeMap[host].RunCommand("sudo rbd snap purge " + name + " && sudo rbd rm " + name)
	}
}

func purgeVolumeHost(t *testing.T, host string, purgeCeph bool) {
	purgeVolume(t, host, host, purgeCeph)
}

func createVolumeHost(t *testing.T, host string) {
	createVolume(t, host, host)
}

func createVolume(t *testing.T, host, name string) {
	t.Logf("Creating %s/%s", host, name)

	if out, err := nodeMap[host].RunCommandWithOutput("docker volume create -d tenant1 --name rbd/" + name); err != nil {
		t.Log(string(out))
		t.Fatal(err)
	}

	if out, err := nodeMap[host].RunCommandWithOutput("sudo rbd ls | grep -q " + name); err != nil {
		t.Log(string(out))
		t.Fatal(err)
	}
}
