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
