package config

import (
	. "testing"

	. "gopkg.in/check.v1"
)

type mergeSuite struct{}

var _ = Suite(&mergeSuite{})

func TestMerge(t *T) { TestingT(t) }

func (s mergeSuite) TestMerge(c *C) {
	v := VolumeOptions{}
	opts := map[string]string{
		"size":                "10",
		"snapshots":           "false",
		"snapshots.frequency": "10m",
		"snapshots.keep":      "20",
	}

	c.Assert(mergeOpts(&v, opts), IsNil)
	c.Assert(v.UseSnapshots, Equals, false)
	c.Assert(v.Size, Equals, uint64(10))
	c.Assert(v.Snapshot.Keep, Equals, uint(20))
	c.Assert(v.Snapshot.Frequency, Equals, "10m")
}
