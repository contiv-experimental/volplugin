package config

import (
	"sort"

	. "gopkg.in/check.v1"
)

var testUseVolumeConfigs = map[string]*VolumeConfig{
	"basic":  &VolumeConfig{TenantName: "tenant1", VolumeName: "quux"},
	"basic2": &VolumeConfig{TenantName: "tenant2", VolumeName: "baz"},
}

var testUseConfigs = map[string]*UseConfig{
	"basic": {
		Volume:   testUseVolumeConfigs["basic"],
		Hostname: "hostname",
	},
	"basic2": {
		Volume:   testUseVolumeConfigs["basic2"],
		Hostname: "hostname",
	},
}

func (s *configSuite) TestUseCRUD(c *C) {
	c.Assert(s.tlc.PublishUse(testUseConfigs["basic"]), IsNil)
	c.Assert(s.tlc.PublishUse(testUseConfigs["basic"]), NotNil)
	c.Assert(s.tlc.RemoveUse(testUseConfigs["basic"], false), IsNil)
	c.Assert(s.tlc.PublishUse(testUseConfigs["basic"]), IsNil)

	mt, err := s.tlc.GetUse(testUseVolumeConfigs["basic"])
	c.Assert(err, IsNil)
	c.Assert(testUseConfigs["basic"], DeepEquals, mt)

	c.Assert(s.tlc.PublishUse(testUseConfigs["basic2"]), IsNil)
	c.Assert(s.tlc.PublishUse(testUseConfigs["basic2"]), NotNil)
	c.Assert(s.tlc.RemoveUse(testUseConfigs["basic2"], false), IsNil)
	c.Assert(s.tlc.PublishUse(testUseConfigs["basic2"]), IsNil)

	mt, err = s.tlc.GetUse(testUseVolumeConfigs["basic2"])
	c.Assert(err, IsNil)
	c.Assert(testUseConfigs["basic2"], DeepEquals, mt)

	mounts, err := s.tlc.ListUses()
	c.Assert(err, IsNil)

	sort.Strings(mounts)
	c.Assert([]string{"tenant1/quux", "tenant2/baz"}, DeepEquals, mounts)
}
