package config

import (
	"sort"

	. "gopkg.in/check.v1"
)

var testMountConfigs = map[string]*MountConfig{
	"basic": {
		Volume:     "quux",
		Pool:       "rbd",
		MountPoint: "/tmp/mountpoint",
		Host:       "hostname",
	},
	"basic2": {
		Volume:     "baz",
		Pool:       "rbd",
		MountPoint: "/tmp/mountpoint",
		Host:       "hostname",
	},
}

func (s configSuite) TestMountCRUD(c *C) {
	c.Assert(s.tlc.PublishMount(testMountConfigs["basic"]), IsNil)
	c.Assert(s.tlc.ExistsMount(testMountConfigs["basic"]), Equals, true)

	defer func() {
		c.Assert(s.tlc.RemoveMount(testMountConfigs["basic"]), IsNil)
		c.Assert(s.tlc.ExistsMount(testMountConfigs["basic"]), Equals, false)
	}()

	mt, err := s.tlc.GetMount("rbd", "quux")
	c.Assert(err, IsNil)
	c.Assert(testMountConfigs["basic"], DeepEquals, mt)

	c.Assert(s.tlc.PublishMount(testMountConfigs["basic2"]), IsNil)
	c.Assert(s.tlc.ExistsMount(testMountConfigs["basic2"]), Equals, true)

	defer func() {
		c.Assert(s.tlc.RemoveMount(testMountConfigs["basic2"]), IsNil)
		c.Assert(s.tlc.ExistsMount(testMountConfigs["basic2"]), Equals, false)
	}()

	mt, err = s.tlc.GetMount("rbd", "baz")
	c.Assert(err, IsNil)
	c.Assert(testMountConfigs["basic2"], DeepEquals, mt)

	mounts, err := s.tlc.ListMounts()
	c.Assert(err, IsNil)

	sort.Strings(mounts)
	c.Assert([]string{"rbd/baz", "rbd/quux"}, DeepEquals, mounts)
}
