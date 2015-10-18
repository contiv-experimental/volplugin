package config

import (
	. "gopkg.in/check.v1"
)

var testTenantConfigs = map[string]*TenantConfig{
	"basic": &TenantConfig{
		DefaultVolumeOptions: VolumeOptions{
			Size:         10,
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		DefaultPool: "rbd",
		FileSystems: defaultFilesystems,
	},
}

func (s *configSuite) TestBasicTenant(c *C) {
	c.Assert(s.tlc.PublishTenant("quux", testTenantConfigs["basic"]), IsNil)

	cfg, err := s.tlc.GetTenant("quux")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testTenantConfigs["basic"])
}
