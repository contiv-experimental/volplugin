package systemtests

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/contiv/volplugin/config"
)

func TestVolCLITenant(t *testing.T) {
	intent1, err := readIntent("testdata/intent1.json")
	if err != nil {
		t.Fatal(err)
	}

	intent2, err := readIntent("testdata/intent2.json")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := volcli("tenant upload test1 < /testdata/intent1.json"); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if _, err := volcli("tenant delete test1"); err != nil {
			t.Fatal(err)
		}

		if _, err := volcli("tenant get test1"); err == nil {
			t.Fatal("Tenant #1 was not actually deleted after deletion command")
		}
	}()

	if _, err := volcli("tenant upload test2 < /testdata/intent2.json"); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if _, err := volcli("tenant delete test2"); err != nil {
			t.Fatal(err)
		}

		if _, err := volcli("tenant get test2"); err == nil {
			t.Fatal("Tenant #2 was not actually deleted after deletion command")
		}
	}()

	out, err := volcli("tenant get test1")
	if err != nil {
		t.Fatal(err)
	}

	intentTarget := &config.TenantConfig{}

	if err := json.Unmarshal([]byte(out), intentTarget); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(intent1, intentTarget) {
		t.Fatal("Intent #1 did not equal retrieved value from etcd")
	}

	out, err = volcli("tenant get test2")
	if err != nil {
		t.Fatal(err)
	}

	intentTarget = &config.TenantConfig{}

	if err := json.Unmarshal([]byte(out), intentTarget); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(intent2, intentTarget) {
		t.Fatal("Intent #2 did not equal retrieved value from etcd")
	}

	out, err = volcli("tenant list")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "test1") {
		t.Fatal("Output from `tenant list` did not include tenant test1")
	}

	if !strings.Contains(out, "test2") {
		t.Fatal("Output from `tenant list` did not include tenant test2")
	}
}

func TestVolCLIVolume(t *testing.T) {
	if err := rebootstrap(); err != nil {
		t.Fatal(err)
	}

	if err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if _, err := volcli("tenant delete tenant1"); err != nil {
			t.Fatal(err)
		}
	}()

	// XXX note that this is removed as a standard part of the tests and may error,
	// so we don't check it.
	defer volcli("volume remove rbd foo")

	if out, err := docker("run --rm --volume-driver tenant1 -v rbd/foo:/mnt ubuntu ls"); err != nil {
		t.Log(out)
		t.Fatal(err)
	}

	out, err := volcli("volume list rbd")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "foo") {
		t.Fatal("Did not find volume after creation")
	}

	out, err = volcli("volume list-pools")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "rbd") {
		t.Fatal(err)
	}

	out, err = volcli("volume get rbd foo")
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.VolumeConfig{}

	if err := json.Unmarshal([]byte(out), cfg); err != nil {
		t.Fatal(err)
	}

	intent1, err := readIntent("testdata/intent1.json")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(intent1.DefaultVolume, cfg) {
		t.Log(intent1.DefaultVolume)
		t.Log(cfg)
		t.Fatal("Tenant configuration did not equal volume configuration, yet no tenant changes were made")
	}

	if _, err := volcli("volume remove rbd foo"); err != nil {
		t.Fatal(err)
	}
}

func TestVolCLIMount(t *testing.T) {
	if err := uploadIntent("tenant1", "intent1"); err != nil {
		t.Fatal(err)
	}

	id, err := docker("run -itd --volume-driver tenant1 -v rbd/foo:/mnt ubuntu sleep infinity")
	if err != nil {
		t.Log(id) // error output
		t.Fatal(err)
	}

	defer volcli("volume remove rbd foo")
	defer docker("volume rm rbd/foo")
	defer docker("rm -f " + id)

	out, err := volcli("mount list")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "foo") {
		t.Fatal("could not find mount")
	}

	out, err = volcli("mount get rbd foo")
	if err != nil {
		t.Fatal(err)
	}

	mt := &config.MountConfig{}

	if err := json.Unmarshal([]byte(out), mt); err != nil {
		t.Fatal(err)
	}

	if mt.Volume != "foo" ||
		mt.Pool != "rbd" ||
		mt.Host != "ceph-mon0" ||
		mt.MountPoint != "/mnt/ceph/rbd/foo" {
		t.Log(mt)
		t.Fatal("Data from mount did not match expectation")
	}

	if _, err := volcli("mount force-remove rbd foo"); err != nil {
		t.Fatal(err)
	}

	out, err = volcli("mount list")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out, "foo") {
		t.Fatal("mount should not exist, still does")
	}

	// the defer comes ahead of time here because of concerns that volume create
	// will half-create a volume
	defer purgeVolume(t, "mon0", "rbd", "foo", true)
	if _, err := volcli("volume create tenant1 rbd foo"); err != nil {
		t.Fatal(err)
	}

	// ensure that double-create does nothing (for now, at least)
	if _, err := volcli("volume create tenant1 rbd foo"); err != nil {
		t.Fatal(err)
	}

	out, err = volcli("volume get rbd foo")
	if err != nil {
		t.Fatal(err)
	}

	// this test should never fail; we should always fail because of an exit code
	// instead, which would happen above.
	if out == "" {
		t.Fatal("Received no infomration")
	}
}
