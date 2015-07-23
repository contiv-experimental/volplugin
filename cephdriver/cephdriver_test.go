package cephdriver

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/librbd"
)

func readWriteTest(mountDir string) error {
	// Write a file and verify you can read it
	file, err := os.Create(mountDir + "/test.txt")
	if err != nil {
		log.Errorf("Error creating file. Err: %v", err)
		return errors.New("Failed to create a file")
	}

	num, err := file.WriteString("Test string\n")
	if err != nil {
		log.Errorf("Error writing file. Err: %v", err)
		return errors.New("Failed to write a file")
	}

	file.Close()

	file, err = os.Open(mountDir + "/test.txt")
	if err != nil {
		log.Errorf("Error opening file. Err: %v", err)
		return errors.New("Failed to open a file")
	}

	rb := make([]byte, 200)
	_, err = io.ReadAtLeast(file, rb, num)
	var rbs = string(rb)
	if (err != nil) || (!strings.Contains(rbs, "Test string")) {
		log.Errorf("Error reading back file(Got %s). Err: %v", rbs, err)
		return errors.New("Failed to read back a file")
	}
	log.Infof("Read back: %s", string(rb))
	file.Close()

	return nil
}

func TestMountUnmountVolume(t *testing.T) {
	config, err := librbd.ReadConfig("/etc/rbdconfig.json")
	if err != nil {
		t.Fatal(err)
	}

	// Create a new driver
	cephDriver, err := NewCephDriver(config, "rbd")
	if err != nil {
		t.Fatal(err)
	}

	volumeSpec := cephDriver.NewVolume("pithos1234", 10240000)

	// we don't care if there's an error here, just want to make sure the create
	// succeeds. Easier restart of failed tests this way.
	volumeSpec.Unmount()
	volumeSpec.Remove()

	if err := volumeSpec.Create(); err != nil {
		t.Fatalf("Error creating the volume: %v", err)
	}

	// mount the volume
	if err := volumeSpec.Mount(); err != nil {
		t.Fatalf("Error mounting the volume. Err: %v", err)
	}

	if err := readWriteTest("/mnt/ceph/rbd/pithos1234"); err != nil {
		t.Fatalf("Error during read/write test. Err: %v", err)
	}

	// unmount the volume
	if err := volumeSpec.Unmount(); err != nil {
		t.Fatalf("Error unmounting the volume. Err: %v", err)
	}

	if err := volumeSpec.Remove(); err != nil {
		t.Fatalf("Error deleting the volume: %v", err)
	}
}

func TestSnapshots(t *testing.T) {
	config, err := librbd.ReadConfig("/etc/rbdconfig.json")
	if err != nil {
		t.Fatal(err)
	}

	// Create a new driver
	cephDriver, err := NewCephDriver(config, "rbd")
	if err != nil {
		t.Fatal(err)
	}

	volumeSpec := cephDriver.NewVolume("pithos1234", 10000000)
	// Create a volume
	if err := volumeSpec.Create(); err != nil {
		t.Fatalf("Error creating the volume. Err: %v", err)
	}

	if err := volumeSpec.CreateSnapshot("hello"); err != nil {
		t.Fatal(err)
	}

	if err := volumeSpec.CreateSnapshot("hello"); err == nil {
		t.Fatal("Was able to create same snapshot name twice")
	}

	list, err := volumeSpec.ListSnapshots(100)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 || !reflect.DeepEqual(list, []string{"hello"}) {
		t.Fatal("Did not see snapshot created earlier in list")
	}

	if err := volumeSpec.RemoveSnapshot("hello"); err != nil {
		t.Fatal(err)
	}

	if err := volumeSpec.RemoveSnapshot("hello"); err == nil {
		t.Fatal("Was able to remove same snapshot name twice")
	}

	list, err = volumeSpec.ListSnapshots(100)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Fatal("Snapshot list is not empty and should be")
	}

	// delete the volume
	if err := volumeSpec.Remove(); err != nil {
		t.Fatalf("Error deleting the volume. Err: %v", err)
	}
}

func TestRepeatedMountUnmount(t *testing.T) {
	config, err := librbd.ReadConfig("/etc/rbdconfig.json")
	if err != nil {
		t.Fatal(err)
	}

	// Create a new driver
	cephDriver, err := NewCephDriver(config, "rbd")
	if err != nil {
		t.Fatal(err)
	}

	volumeSpec := cephDriver.NewVolume("pithos1234", 10000000)
	// Create a volume
	if err := volumeSpec.Create(); err != nil {
		t.Fatalf("Error creating the volume. Err: %v", err)
	}

	// Repeatedly perform mount unmount test
	for i := 0; i < 10; i++ {
		// mount the volume
		if err := volumeSpec.Mount(); err != nil {
			t.Fatalf("Error mounting the volume. Err: %v", err)
		}

		if err := readWriteTest("/mnt/ceph/rbd/pithos1234"); err != nil {
			t.Fatalf("Error during read/write test. Err: %v", err)
		}

		// unmount the volume
		if err := volumeSpec.Unmount(); err != nil {
			t.Fatalf("Error unmounting the volume. Err: %v", err)
		}
	}

	// delete the volume
	if err := volumeSpec.Remove(); err != nil {
		t.Fatalf("Error deleting the volume. Err: %v", err)
	}
}
