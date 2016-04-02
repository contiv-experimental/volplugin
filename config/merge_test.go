package config

import (
	. "gopkg.in/check.v1"
)

func (s *configSuite) TestMerge(c *C) {
	v := RuntimeOptions{}
	opts := map[string]string{
		"snapshots":           "false",
		"snapshots.frequency": "10m",
		"snapshots.keep":      "20",
	}

	c.Assert(mergeOpts(&v, opts), IsNil)
	c.Assert(v.UseSnapshots, Equals, false)
	c.Assert(v.Snapshot.Keep, Equals, uint(20))
	c.Assert(v.Snapshot.Frequency, Equals, "10m")
}
