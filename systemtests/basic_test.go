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

func TestVolumeCreateMultiHost(t *testing.T) {
	hosts := []string{"mon0", "mon1", "mon2"}
	defer func() {
		for _, host := range hosts {
			t.Logf("Purging %s", host)

			if err := nodeMap[host].RunCommand("docker volume rm " + host); err != nil {
				t.Fatal(err)
			}

			if err := nodeMap[host].RunCommand("sudo rbd snap purge " + host + " && sudo rbd rm " + host); err != nil {
				t.Fatal(err)
			}
		}
	}()

	for _, host := range []string{"mon0", "mon1", "mon2"} {
		t.Logf("Creating %s", host)

		if err := nodeMap[host].RunCommand("docker volume create -d tenant1 --name " + host); err != nil {
			t.Fatal(err)
		}

		if err := nodeMap[host].RunCommand("sudo rbd list | grep -q " + host); err != nil {
			t.Fatal(err)
		}
	}
}
