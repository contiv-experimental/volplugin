package librbd

import (
	"os"
	"reflect"
	"testing"
)

func TestVersion(t *testing.T) {
	t.Log(Version())
}

func TestPool(t *testing.T) {
	config, err := ReadConfig("/etc/rbdconfig.json")
	if err != nil {
		t.Fatal(err)
	}

	pool, err := GetPool(config, "rbd")
	if err != nil {
		t.Fatal(err)
	}

	if pool.ioctx == nil {
		t.Fatal("ioctx was nil")
	}

	if pool.cluster == nil {
		t.Fatal("rados was nil")
	}

	// it's ok if it fails, we just don't want the next call to unless stuff is
	// broken.
	pool.RemoveImage("test")
	if err := pool.CreateImage("test", 10000000); err != nil {
		t.Fatal(err)
	}

	if err := pool.CreateImage("test", 10000000); err == nil {
		t.Fatal("No error was recieved trying to create image twice")
	}

	defer pool.RemoveImage("test")

	items, err := pool.List()

	if err != nil {
		t.Fatal(err)
	}

	var found bool

	for _, item := range items {
		if item == "test" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("image list was invalid")
	}

	device, err := pool.MapDevice("test")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(device); err != nil {
		t.Fatal(err)
	}

	list, err := pool.ListSnapshots("test", 100)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Fatalf("Snapshot list expected to be empty but is not: %v", list)
	}

	if err := pool.CreateSnapshot("test", "test-snap"); err != nil {
		t.Fatal(err)
	}

	if err := pool.CreateSnapshot("test", "test-snap"); err == nil {
		t.Fatal("Did not recieve error creating snapshot twice")
	}

	defer pool.RemoveSnapshot("test", "test-snap")

	if list, err = pool.ListSnapshots("test", 100); err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 || !reflect.DeepEqual(list, []string{"test-snap"}) {
		t.Fatal("Snapshot list after create did not match expectation")
	}

	if err := pool.RemoveSnapshot("test", "test-snap"); err != nil {
		t.Fatal(err)
	}

	if err := pool.RemoveSnapshot("test", "test-snap"); err == nil {
		t.Fatal("Did not receive error when trying to delete the same snapshot twice")
	}

	if list, err = pool.ListSnapshots("test", 100); err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Fatal("Snapshot list after create did not match expectation")
	}

	device2, err := pool.MapDevice("test")
	if err != nil {
		t.Fatal(err)
	}

	if device != device2 {
		t.Fatal("mapdevice failed to find existing rbd device")
	}

	if err := pool.UnmapDevice("test"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(device); err == nil {
		t.Fatal("device still exists after unmap")
	}

	if err := pool.UnmapDevice("test"); err == nil {
		t.Fatal("Did not receive error trying to unmap device twice")
	}

	if err := pool.RemoveImage("test"); err != nil {
		t.Fatal(err)
	}

	if err := pool.RemoveImage("test"); err == nil {
		t.Fatal("Did not receive error trying to remove image a second time")
	}
}
