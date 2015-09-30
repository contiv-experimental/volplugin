package config

import "testing"

func TestMerge(t *testing.T) {
	v := VolumeOptions{}
	if err := mergeOpts(&v, map[string]string{"size": "10", "snapshots.frequency": "10m", "snapshots.keep": "20"}); err != nil {
		t.Fatal(err)
	}

	if v.Size != 10 {
		t.Fatal("size was not populated according to schema")
	}

	if v.Snapshot.Keep != 20 {
		t.Fatal("snapshots.keep was not populated according to schema")
	}

	if v.Snapshot.Frequency != "10m" {
		t.Fatal("snapshots.frequency was not populated according to schema")
	}
}
