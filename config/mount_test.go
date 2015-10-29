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

func (s *configSuite) TestMountCRUD(c *C) {
	c.Assert(s.tlc.PublishMount(testMountConfigs["basic"]), IsNil)
	c.Assert(s.tlc.PublishMount(testMountConfigs["basic"]), NotNil)
	c.Assert(s.tlc.RemoveMount(testMountConfigs["basic"], false), IsNil)
	c.Assert(s.tlc.PublishMount(testMountConfigs["basic"]), IsNil)

	mt, err := s.tlc.GetMount("rbd", "quux")
	c.Assert(err, IsNil)
	c.Assert(testMountConfigs["basic"], DeepEquals, mt)

	c.Assert(s.tlc.PublishMount(testMountConfigs["basic2"]), IsNil)
	c.Assert(s.tlc.PublishMount(testMountConfigs["basic2"]), NotNil)
	c.Assert(s.tlc.RemoveMount(testMountConfigs["basic2"], false), IsNil)
	c.Assert(s.tlc.PublishMount(testMountConfigs["basic2"]), IsNil)

	mt, err = s.tlc.GetMount("rbd", "baz")
	c.Assert(err, IsNil)
	c.Assert(testMountConfigs["basic2"], DeepEquals, mt)

	mounts, err := s.tlc.ListMounts()
	c.Assert(err, IsNil)

	sort.Strings(mounts)
	c.Assert([]string{"rbd/baz", "rbd/quux"}, DeepEquals, mounts)
}
