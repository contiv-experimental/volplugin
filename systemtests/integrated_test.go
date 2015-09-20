package systemtests

import (
	"strings"
	"testing"
	"time"
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
