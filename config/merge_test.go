package config

import (
	. "gopkg.in/check.v1"
)

func (s *configSuite) TestMerge(c *C) {
	p := &Policy{}
	opts := map[string]string{
		"size":                "200MB",
		"snapshots":           "false",
		"snapshots.frequency": "10m",
		"snapshots.keep":      "20",
	}

	c.Assert(mergeOpts(p, opts), IsNil)
	actualSize, err := p.CreateOptions.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(actualSize, Equals, uint64(200))
	c.Assert(p.RuntimeOptions.UseSnapshots, Equals, false)
	c.Assert(p.RuntimeOptions.Snapshot.Keep, Equals, uint(20))
	c.Assert(p.RuntimeOptions.Snapshot.Frequency, Equals, "10m")
}
