package systemtests

import "testing"

func TestEtcdUpdate(t *testing.T) {
	// this not-very-obvious test ensures that the tenant can be uploaded after
	// the volplugin/volmaster pair are started.
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Fatal(err)
	}

	if _, err := docker("volume create -d tenant1 --name rbd/foo"); err != nil {
		t.Fatal(err)
	}

	defer volcli("volume remove rbd foo")
	defer docker("volume rm rbd/foo")
}
