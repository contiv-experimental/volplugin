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

	if err := createVolume("mon0", "rbd", "foo"); err != nil {
		t.Fatal(err)
	}
	purgeVolume("mon0", "rbd", "foo", true)
}

func TestSnapshotSchedule(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if err := uploadIntent("tenant1", "fastsnap"); err != nil {
		t.Fatal(err)
	}

	if err := createVolume("mon0", "rbd", "foo"); err != nil {
		t.Fatal(err)
	}
	defer purgeVolume("mon0", "rbd", "foo", true)
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

	if err := stopVolplugin(); err != nil {
		t.Fatal(err)
	}

	if _, err := nodeMap["mon0"].RunCommandBackground("sudo -E `which volplugin` --host-label quux --debug tenant1"); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Fatal(err)
	}

	out, err := docker("run -d --volume-driver tenant1 -v rbd/foo:/mnt ubuntu sleep infinity")
	if err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	defer purgeVolume("mon0", "rbd", "foo", true)
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
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Fatal(err)
	}

	if err := createVolume("mon0", "rbd", "test"); err != nil {
		t.Fatal(err)
	}
	defer purgeVolume("mon0", "rbd", "test", true)
	defer purgeVolume("mon1", "rbd", "test", false)
	defer purgeVolume("mon2", "rbd", "test", false)
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

	purgeVolume("mon0", "rbd", "test", false)

	// Repeat the test to ensure it's working cross-host.

	if err := nodeMap["mon1"].RunCommand(dockerCmd); err != nil {
		t.Fatal(err)
	}

	defer purgeVolume("mon1", "rbd", "test", false)

	for _, nodeName := range []string{"mon0", "mon2"} {
		if out, err := nodeMap[nodeName].RunCommandWithOutput(dockerCmd); err == nil {
			t.Log(out)
			t.Fatalf("%s was able to mount while mon0 held the mount", nodeName)
		}
	}
}

func TestMultiPool(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Fatal(err)
	}

	if out, err := mon0cmd("sudo ceph osd pool create test 1 1"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	defer mon0cmd("sudo ceph osd pool delete test test --yes-i-really-really-mean-it")

	if err := createVolume("mon0", "test", "test"); err != nil {
		t.Fatal(err)
	}
	defer purgeVolume("mon0", "test", "test", true)

	out, err := volcli("volume get test test")
	if err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	vc := &config.VolumeConfig{}
	if err := json.Unmarshal([]byte(out), vc); err != nil {
		t.Fatal(err)
	}

	if vc.Size != 10 {
		t.Logf("%#v", *vc)
		t.Fatal("Could not retrieve properties from volume")
	}
}
