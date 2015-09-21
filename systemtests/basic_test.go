package systemtests

import (
	"strings"
	"testing"
)

func TestStarted(t *testing.T) {
	if err := nodeMap["mon0"].RunCommand("pgrep -c volmaster"); err != nil {
		t.Fatal(err)
	}

	if err := runSSH("pgrep -c volplugin"); err != nil {
		t.Fatal(err)
	}
}

func TestSSH(t *testing.T) {
	if err := runSSH("/bin/echo"); err != nil {
		t.Fatal(err)
	}
}

func TestVolumeCreate(t *testing.T) {
	defer purgeVolumeHost(t, "mon0", true)
	createVolumeHost(t, "mon0")
}

func TestVolumeCreateMultiHost(t *testing.T) {
	hosts := []string{"mon0", "mon1", "mon2"}
	defer func() {
		for _, host := range hosts {
			purgeVolumeHost(t, host, true)
		}
	}()

	for _, host := range []string{"mon0", "mon1", "mon2"} {
		createVolumeHost(t, host)
	}
}

func TestVolumeCreateMultiHostCrossHostMount(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Fatal(err)
	}

	createVolume(t, "mon0", "test")
	if out, err := nodeMap["mon0"].RunCommandWithOutput(`docker run --rm -i -v rbd/test:/mnt ubuntu sh -c "echo bar >/mnt/foo"`); err != nil {
		t.Log(out)
		purgeVolume(t, "mon0", "test", true) // cleanup
		t.Fatal(err)
	}

	createVolume(t, "mon1", "test")

	out, err := nodeMap["mon1"].RunCommandWithOutput(`docker run --rm -i -v rbd/test:/mnt ubuntu sh -c "cat /mnt/foo"`)
	if err != nil {
		t.Log(out)
		purgeVolume(t, "mon1", "test", true) // cleanup
		t.Fatal(err)
	}

	if strings.TrimSpace(out) != "bar" {
		t.Fatalf("output did not equal expected result: %q", out)
	}

	createVolume(t, "mon1", "test")

	if out, err := nodeMap["mon1"].RunCommandWithOutput(`docker run --rm -i -v rbd/test:/mnt ubuntu sh -c "echo quux >/mnt/foo"`); err != nil {
		t.Log(out)
		purgeVolume(t, "mon1", "test", true) // cleanup
		t.Fatal(err)
	}

	createVolume(t, "mon2", "test")
	defer purgeVolume(t, "mon2", "test", true)

	out, err = nodeMap["mon2"].RunCommandWithOutput(`docker run --rm -i -v rbd/test:/mnt ubuntu sh -c "cat /mnt/foo"`)
	if err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if strings.TrimSpace(out) != "quux" {
		t.Fatalf("output did not equal expected result: %q", out)
	}
}
