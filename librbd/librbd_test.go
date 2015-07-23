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

	// FIXME finish
	img, err := pool.GetImage("test")
	if err != nil {
		t.Fatal(err)
	}

	if img.imageName != "test" {
		t.Fatal("Image name was not set properly")
	}

	if !reflect.DeepEqual(img.pool, pool) {
		t.Fatal("Pool was not equal in image struct")
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

	device, err := img.MapDevice()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(device); err != nil {
		t.Fatal(err)
	}

	list, err := img.ListSnapshots(100)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Fatalf("Snapshot list expected to be empty but is not: %v", list)
	}

	if err := img.CreateSnapshot("test-snap"); err != nil {
		t.Fatal(err)
	}

	if err := img.CreateSnapshot("test-snap"); err == nil {
		t.Fatal("Did not recieve error creating snapshot twice")
	}

	defer img.RemoveSnapshot("test-snap")

	if list, err = img.ListSnapshots(100); err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 || !reflect.DeepEqual(list, []string{"test-snap"}) {
		t.Fatal("Snapshot list after create did not match expectation")
	}

	if err := img.RemoveSnapshot("test-snap"); err != nil {
		t.Fatal(err)
	}

	if err := img.RemoveSnapshot("test-snap"); err == nil {
		t.Fatal("Did not receive error when trying to delete the same snapshot twice")
	}

	if list, err = img.ListSnapshots(100); err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Fatal("Snapshot list after create did not match expectation")
	}

	device2, err := img.MapDevice()
	if err != nil {
		t.Fatal(err)
	}

	if device != device2 {
		t.Fatal("mapdevice failed to find existing rbd device")
	}

	if err := img.UnmapDevice(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(device); err == nil {
		t.Fatal("device still exists after unmap")
	}

	if err := img.UnmapDevice(); err == nil {
		t.Fatal("Did not receive error trying to unmap device twice")
	}

	if err := pool.RemoveImage("test"); err != nil {
		t.Fatal(err)
	}

	if err := pool.RemoveImage("test"); err == nil {
		t.Fatal("Did not receive error trying to remove image a second time")
	}

	if _, err := pool.GetImage("test"); err == nil {
		t.Fatal("Was able to get image after removal")
	}
}
