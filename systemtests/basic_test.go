package systemtests

import "testing"

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
	volumeName := "volumeCreate"

	defer func() {
		if err := nodeMap["mon0"].RunCommand("docker volume rm " + volumeName); err != nil {
			t.Fatal(err)
		}

		if err := nodeMap["mon0"].RunCommand("sudo rbd snap purge " + volumeName + " && sudo rbd rm " + volumeName); err != nil {
			t.Fatal(err)
		}
	}()

	if err := nodeMap["mon0"].RunCommand("docker volume create -d tenant1 --name " + volumeName); err != nil {
		t.Fatal(err)
	}

	if err := nodeMap["mon0"].RunCommand("sudo rbd list | grep -q " + volumeName); err != nil {
		t.Fatal(err)
	}
}
