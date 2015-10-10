package systemtests

import (
	"strings"
	. "testing"
)

func TestStarted(t *T) {
	if err := nodeMap["mon0"].RunCommand("pgrep -c volmaster"); err != nil {
		t.Fatal(err)
	}

	if err := runSSH("pgrep -c volplugin"); err != nil {
		t.Fatal(err)
	}
}

func TestSSH(t *T) {
	if err := runSSH("/bin/echo"); err != nil {
		t.Fatal(err)
	}
}

func TestVolumeCreate(t *T) {
	defer purgeVolumeHost("tenant1", "mon0", true)
	if err := createVolumeHost("tenant1", "mon0", nil); err != nil {
		t.Fatal(err)
	}
}

func TestVolumeCreateMultiHost(t *T) {
	hosts := []string{"mon0", "mon1", "mon2"}
	defer func() {
		for _, host := range hosts {
			purgeVolumeHost("tenant1", host, true)
		}
	}()

	for _, host := range []string{"mon0", "mon1", "mon2"} {
		if err := createVolumeHost("tenant1", host, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVolumeCreateMultiHostCrossHostMount(t *T) {
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

	if out, err := nodeMap["mon0"].RunCommandWithOutput(`docker run --rm -i -v tenant1/test:/mnt ubuntu sh -c "echo bar >/mnt/foo"`); err != nil {
		t.Log(out)
		purgeVolume("mon0", "tenant1", "test", true) // cleanup
		t.Fatal(err)
	}

	if err := createVolume("mon1", "tenant1", "test", nil); err != nil {
		t.Fatal(err)
	}

	out, err := nodeMap["mon1"].RunCommandWithOutput(`docker run --rm -i -v tenant1/test:/mnt ubuntu sh -c "cat /mnt/foo"`)
	if err != nil {
		t.Log(out)
		purgeVolume("mon1", "tenant1", "test", true) // cleanup
		t.Fatal(err)
	}

	if strings.TrimSpace(out) != "bar" {
		t.Fatalf("output did not equal expected result: %q", out)
	}

	if err := createVolume("mon1", "tenant1", "test", nil); err != nil {
		t.Fatal(err)
	}

	if out, err := nodeMap["mon1"].RunCommandWithOutput(`docker run --rm -i -v tenant1/test:/mnt ubuntu sh -c "echo quux >/mnt/foo"`); err != nil {
		t.Log(out)
		purgeVolume("mon1", "tenant1", "test", true) // cleanup
		t.Fatal(err)
	}

	if err := createVolume("mon2", "tenant1", "test", nil); err != nil {
		t.Fatal(err)
	}
	defer purgeVolume("mon2", "tenant1", "test", true)

	out, err = nodeMap["mon2"].RunCommandWithOutput(`docker run --rm -i -v tenant1/test:/mnt ubuntu sh -c "cat /mnt/foo"`)
	if err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	if strings.TrimSpace(out) != "quux" {
		t.Fatalf("output did not equal expected result: %q", out)
	}
}
