package librbd

import (
	"os"
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

	defer func() {
		if err := pool.RemoveImage("test"); err != nil {
			t.Fatal(err)
		}
	}()

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

	// FIXME test this was actually done, not just the error
	// if err := pool.CreateSnapshot("test", "test-snap"); err != nil {
	// 	t.Fatal(err)
	// }

	defer func() {
		if err := pool.UnmapDevice("test"); err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat(device); err == nil {
			t.Fatal("device still exists after unmap")
		}
	}()

	device2, err := pool.MapDevice("test")
	if err != nil {
		t.Fatal(err)
	}

	if device != device2 {
		t.Fatal("mapdevice failed to find existing rbd device")
	}
}
