package config

import . "gopkg.in/check.v1"

var testTenantConfigs = map[string]*TenantConfig{
	"basic": {
		DefaultVolumeOptions: VolumeOptions{
			Pool:         "rbd",
			Size:         "10MB",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"basic2": {
		DefaultVolumeOptions: VolumeOptions{
			Pool:         "rbd",
			Size:         "20MB",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"nopool": {
		DefaultVolumeOptions: VolumeOptions{
			Size:         "20MB",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsize": {
		DefaultVolumeOptions: VolumeOptions{
			Size:         "0",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsize2": {
		DefaultVolumeOptions: VolumeOptions{
			Size:         "10M",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsize3": {
		DefaultVolumeOptions: VolumeOptions{
			Size:         "not a number",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
}

func (s *configSuite) TestBasicTenant(c *C) {
	c.Assert(s.tlc.PublishTenant("quux", testTenantConfigs["basic"]), IsNil)

	cfg, err := s.tlc.GetTenant("quux")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testTenantConfigs["basic"])

	c.Assert(s.tlc.PublishTenant("bar", testTenantConfigs["basic2"]), IsNil)

	cfg, err = s.tlc.GetTenant("bar")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testTenantConfigs["basic2"])

	tenants, err := s.tlc.ListTenants()
	c.Assert(err, IsNil)

	for _, tenant := range tenants {
		found := false
		for _, name := range []string{"bar", "quux"} {
			if tenant == name {
				found = true
			}
		}

		c.Assert(found, Equals, true)
	}

	c.Assert(s.tlc.DeleteTenant("bar"), IsNil)
	_, err = s.tlc.GetTenant("bar")
	c.Assert(err, NotNil)

	cfg, err = s.tlc.GetTenant("quux")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testTenantConfigs["basic"])
}

func (s *configSuite) TestTenantValidate(c *C) {
	for _, key := range []string{"basic", "basic2"} {
		c.Assert(testTenantConfigs[key].Validate(), IsNil)
	}

	c.Assert(testTenantConfigs["nopool"].Validate(), NotNil)
	c.Assert(testTenantConfigs["badsize"].Validate(), NotNil)
	c.Assert(testTenantConfigs["badsize2"].Validate(), NotNil)
	_, err := testTenantConfigs["badsize3"].DefaultVolumeOptions.ActualSize()
	c.Assert(err, NotNil)
}
