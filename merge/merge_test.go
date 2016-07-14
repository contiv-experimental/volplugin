package merge

import (
	. "testing"

	. "gopkg.in/check.v1"
)

type mergeSuite struct{}

var _ = Suite(&mergeSuite{})

func TestMerge(t *T) { TestingT(t) }

type mergeExample struct {
	Size           string `merge:"size"`
	FileSystem     string `merge:"filesystem"`
	Unlocked       bool   `merge:"unlocked"`
	RuntimeOptions struct {
		UseSnapshots bool `merge:"snapshots"`
		Snapshot     struct {
			Frequency string `merge:"snapshots.frequency"`
			Keep      uint   `merge:"snapshots.keep"`
		}
	}
}

func (s *mergeSuite) TestMerge(c *C) {
	v := &mergeExample{}
	opts := map[string]string{
		"size":                "200MB",
		"snapshots":           "false",
		"snapshots.frequency": "10m",
		"snapshots.keep":      "20",
		"unlocked":            "true",
	}

	c.Assert(Opts(v, opts), IsNil)
	c.Assert(v.Size, Equals, "200MB")
	c.Assert(v.RuntimeOptions.UseSnapshots, Equals, false)
	c.Assert(v.RuntimeOptions.Snapshot.Keep, Equals, uint(20))
	c.Assert(v.RuntimeOptions.Snapshot.Frequency, Equals, "10m")
	c.Assert(v.Unlocked, Equals, true)
}
