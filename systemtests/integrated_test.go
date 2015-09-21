package systemtests

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/contiv/volplugin/config"
)

func TestEtcdUpdate(t *testing.T) {
	// this not-very-obvious test ensures that the tenant can be uploaded after
	// the volplugin/volmaster pair are started.
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Fatal(err)
	}

	createVolume(t, "mon0", "foo")
	purgeVolume(t, "mon0", "foo", true)
}

func TestSnapshotSchedule(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if err := uploadIntent("tenant1", "fastsnap"); err != nil {
		t.Fatal(err)
	}

	createVolume(t, "mon0", "foo")
	defer purgeVolume(t, "mon0", "foo", true)
	defer rebootstrap()

	time.Sleep(2 * time.Second)

	out, err := nodeMap["mon0"].RunCommandWithOutput("sudo rbd snap ls foo")
	if err != nil {
		t.Fatal(err)
	}

	if len(strings.TrimSpace(out)) == 0 {
		t.Log(out)
		t.Fatal("Could not find the right number of snapshots for the volume")
	}
}

func TestHostLabel(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	defer rebootstrap()

	if err := stopVolplugin(); err != nil {
		t.Fatal(err)
	}

	if _, err := nodeMap["mon0"].RunCommandBackground("sudo -E `which volplugin` --host-label quux --debug tenant1"); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1 * time.Second)

	if err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Fatal(err)
	}

	out, err := docker("run -d --volume-driver tenant1 -v rbd/foo:/mnt ubuntu sleep infinity")
	if err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	defer purgeVolume(t, "mon0", "foo", true)
	defer docker("rm -f " + out)

	mt := &config.MountConfig{}

	out, err = volcli("mount get rbd foo")
	if err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if err := json.Unmarshal([]byte(out), mt); err != nil {
		t.Fatal(err)
	}

	if mt.Host != "quux" {
		t.Fatal("host-label did not propogate")
	}
}

func TestMountLock(t *testing.T) {
	if err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Fatal(err)
	}

	createVolume(t, "mon0", "test")
	defer purgeVolume(t, "mon0", "test", true)
	defer purgeVolume(t, "mon1", "test", false)
	defer purgeVolume(t, "mon2", "test", false)
	defer clearContainers()

	dockerCmd := "docker run -d --volume-driver tenant1 -v rbd/test:/mnt ubuntu sleep infinity"
	if err := nodeMap["mon0"].RunCommand(dockerCmd); err != nil {
		t.Fatal(err)
	}

	for _, nodeName := range []string{"mon1", "mon2"} {
		if out, err := nodeMap[nodeName].RunCommandWithOutput(dockerCmd); err == nil {
			t.Log(out)
			t.Fatalf("%s was able to mount while mon0 held the mount", nodeName)
		}
	}

	if err := clearContainers(); err != nil {
		t.Fatal(err)
	}

	purgeVolume(t, "mon0", "test", false)

	// Repeat the test to ensure it's working cross-host.

	if err := nodeMap["mon1"].RunCommand(dockerCmd); err != nil {
		t.Fatal(err)
	}

	defer purgeVolume(t, "mon1", "test", false)

	for _, nodeName := range []string{"mon0", "mon2"} {
		if out, err := nodeMap[nodeName].RunCommandWithOutput(dockerCmd); err == nil {
			t.Log(out)
			t.Fatalf("%s was able to mount while mon0 held the mount", nodeName)
		}
	}
}
