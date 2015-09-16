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

func TestVolCLIVolume(t *testing.T) {}
func TestVolCLIMount(t *testing.T)  {}
