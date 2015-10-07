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

	if out, err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if err := createVolume("mon0", "tenant1", "foo", nil); err != nil {
		t.Fatal(err)
	}
	purgeVolume("mon0", "tenant1", "foo", true)
}

func TestSnapshotSchedule(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if out, err := uploadIntent("tenant1", "fastsnap"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if err := createVolume("mon0", "tenant1", "foo", nil); err != nil {
		t.Fatal(err)
	}
	defer purgeVolume("mon0", "tenant1", "foo", true)
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

	time.Sleep(100 * time.Millisecond)

	if err := stopVolplugin(); err != nil {
		t.Fatal(err)
	}

	if _, err := nodeMap["mon0"].RunCommandBackground("sudo -E `which volplugin` --host-label quux --debug tenant1"); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	if out, err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if err := createVolume("mon0", "tenant1", "foo", nil); err != nil {
		t.Fatal(err)
	}

	out, err := docker("run -d -v tenant1/foo:/mnt ubuntu sleep infinity")
	if err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	defer purgeVolume("mon0", "tenant1", "foo", true)
	defer docker("rm -f " + out)

	mt := &config.MountConfig{}

	// we know the pool is rbd here, so cheat a little.
	out, err = volcli("mount get rbd foo")
	if err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if err := json.Unmarshal([]byte(out), mt); err != nil {
		t.Fatal(err)
	}

	if mt.Host != "quux" {
		t.Fatal("host-label did not propagate")
	}
}

func TestMountLock(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if out, err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if err := createVolume("mon0", "tenant1", "test", nil); err != nil {
		t.Fatal(err)
	}

	defer purgeVolume("mon0", "tenant1", "test", true)

	for _, name := range []string{"mon1", "mon2"} {
		if err := createVolume(name, "tenant1", "test", nil); err != nil {
			t.Fatal(err)
		}
		defer purgeVolume(name, "tenant1", "test", false)
	}

	defer clearContainers()

	dockerCmd := "docker run -d -v tenant1/test:/mnt ubuntu sleep infinity"
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

	// Repeat the test to ensure it's working cross-host.
	if err := nodeMap["mon1"].RunCommand(dockerCmd); err != nil {
		t.Fatal(err)
	}

	defer purgeVolume("mon1", "tenant1", "test", false)

	for _, nodeName := range []string{"mon0", "mon2"} {
		if out, err := nodeMap[nodeName].RunCommandWithOutput(dockerCmd); err == nil {
			t.Log(out)
			t.Fatalf("%s was able to mount while mon1 held the mount", nodeName)
		}
	}
}

func TestMultiPool(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if out, err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if out, err := mon0cmd("sudo ceph osd pool create test 1 1"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	defer mon0cmd("sudo ceph osd pool delete test test --yes-i-really-really-mean-it")

	if err := createVolume("mon0", "tenant1", "test", map[string]string{"pool": "test"}); err != nil {
		t.Fatal(err)
	}
	defer purgeVolume("mon0", "tenant1", "test", true)

	out, err := volcli("volume get tenant1 test")
	if err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	vc := &config.VolumeConfig{}
	if err := json.Unmarshal([]byte(out), vc); err != nil {
		t.Fatal(err)
	}

	if vc.Options.Size != 10 {
		t.Logf("%#v", *vc)
		t.Fatal("Could not retrieve properties from volume")
	}
}

func TestDriverOptions(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if out, err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	opts := map[string]string{
		"size":                "200",
		"snapshots":           "true",
		"snapshots.frequency": "100m",
		"snapshots.keep":      "20",
	}

	if err := createVolume("mon0", "tenant1", "test", opts); err != nil {
		t.Fatal(err)
	}

	defer purgeVolume("mon0", "tenant1", "test", true)

	out, err := volcli("volume get tenant1 test")
	if err != nil {
		t.Fatal(err)
	}

	vc := &config.VolumeConfig{}
	if err := json.Unmarshal([]byte(out), vc); err != nil {
		t.Fatal(err)
	}

	if vc.Options.Size != 200 {
		t.Logf("%#v", *vc)
		t.Fatal("Size option passed to docker volume create did not propagate to volume options")
	}

	if vc.Options.Snapshot.Frequency != "100m" {
		t.Logf("%#v", *vc)
		t.Fatal("Snapshot Frequency option passed to docker volume create did not propagate to volume options")
	}

	if vc.Options.Snapshot.Keep != 20 {
		t.Logf("%#v", *vc)
		t.Fatal("Size option passed to docker volume create did not propagate to volume options")
	}
}

func TestMultipleFileSystems(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if out, err := uploadIntent("tenant1", "fs"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	opts := map[string]string{
		"size": "1000",
	}

	if err := createVolume("mon0", "tenant1", "test", opts); err != nil {
		t.Fatal(err)
	}

	defer purgeVolume("mon0", "tenant1", "test", true)

	if err := nodeMap["mon0"].RunCommand("docker run -d -v tenant1/test:/mnt ubuntu sleep infinity"); err != nil {
		t.Fatal(err)
	}

	defer clearContainers()

	out, err := nodeMap["mon0"].RunCommandWithOutput("mount -l -t btrfs")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(out, "\n")
	pass := false
	for _, line := range lines {
		// cheat.
		if strings.Contains(line, "/dev/rbd") {
			pass = true
			break
		}
	}

	if !pass {
		t.Fatal("could not find the mounted volume as btrfs")
	}

	if err := createVolume("mon0", "tenant1", "testext4", map[string]string{"filesystem": "ext4"}); err != nil {
		t.Fatal(err)
	}

	defer purgeVolume("mon0", "tenant1", "testext4", true)

	if err := nodeMap["mon0"].RunCommand("docker run -d -v tenant1/testext4:/mnt ubuntu sleep infinity"); err != nil {
		t.Fatal(err)
	}

	out, err = nodeMap["mon0"].RunCommandWithOutput("mount -l -t ext4")
	if err != nil {
		t.Fatal(err)
	}

	lines = strings.Split(out, "\n")
	pass = false
	for _, line := range lines {
		// cheat.
		if strings.Contains(line, "/dev/rbd") {
			pass = true
			break
		}
	}
}

func TestMultiTenantVolumeCreate(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if out, err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if out, err := uploadIntent("tenant2", "intent2"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if err := createVolume("mon0", "tenant1", "test", nil); err != nil {
		t.Fatal(err)
	}

	if err := createVolume("mon0", "tenant2", "test", nil); err != nil {
		t.Fatal(err)
	}

	defer purgeVolume("mon0", "tenant1", "test", true)
	defer purgeVolume("mon0", "tenant2", "test", true)

	if out, err := docker("run -v tenant1/test:/mnt ubuntu sh -c \"echo foo > /mnt/bar\""); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if err := clearContainers(); err != nil {
		t.Fatal(err)
	}

	out, err := docker("run -v tenant2/test:/mnt ubuntu sh -c \"cat /mnt/bar\"")
	if err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if strings.TrimSpace(out) != "foo" {
		t.Log(out)
		t.Fatal("Could not retrieve value set by other tenant")
	}
}
