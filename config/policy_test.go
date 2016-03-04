package config

import . "gopkg.in/check.v1"

var testPolicyConfigs = map[string]*PolicyConfig{
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
	"untouchedwithzerosize": {
		DefaultVolumeOptions: VolumeOptions{
			Pool:         "rbd",
			Size:         "0",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"nilfs": {
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
	"badsnaps": {
		DefaultVolumeOptions: VolumeOptions{
			Size:         "10M",
			UseSnapshots: true,
			FileSystem:   defaultFilesystem,
			Snapshot: SnapshotConfig{
				Keep:      0,
				Frequency: "",
			},
		},
		FileSystems: defaultFilesystems,
	},
}

func (s *configSuite) TestBasicPolicy(c *C) {
	c.Assert(s.tlc.PublishPolicy("quux", testPolicyConfigs["basic"]), IsNil)

	cfg, err := s.tlc.GetPolicy("quux")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testPolicyConfigs["basic"])

	c.Assert(s.tlc.PublishPolicy("bar", testPolicyConfigs["basic2"]), IsNil)

	cfg, err = s.tlc.GetPolicy("bar")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testPolicyConfigs["basic2"])

	policies, err := s.tlc.ListPolicies()
	c.Assert(err, IsNil)

	for _, policy := range policies {
		found := false
		for _, name := range []string{"bar", "quux"} {
			if policy == name {
				found = true
			}
		}

		c.Assert(found, Equals, true)
	}

	c.Assert(s.tlc.DeletePolicy("bar"), IsNil)
	_, err = s.tlc.GetPolicy("bar")
	c.Assert(err, NotNil)

	cfg, err = s.tlc.GetPolicy("quux")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testPolicyConfigs["basic"])
}

func (s *configSuite) TestPolicyValidate(c *C) {
	for _, key := range []string{"basic", "basic2", "nilfs"} {
		c.Assert(testPolicyConfigs[key].Validate(), IsNil)
	}

	// FIXME: ensure the default filesystem option is set when validate is called.
	//        honestly, this both a pretty lousy way to do it and test it, we should do
	//        something better.
	c.Assert(testPolicyConfigs["nilfs"].FileSystems, DeepEquals, map[string]string{defaultFilesystem: "mkfs.ext4 -m0 %"})

	c.Assert(testPolicyConfigs["untouchedwithzerosize"].Validate(), NotNil)
	c.Assert(testPolicyConfigs["nopool"].Validate(), NotNil)
	c.Assert(testPolicyConfigs["badsize"].Validate(), NotNil)
	c.Assert(testPolicyConfigs["badsize2"].Validate(), NotNil)
	_, err := testPolicyConfigs["badsize3"].DefaultVolumeOptions.ActualSize()
	c.Assert(err, NotNil)
}

func (s *configSuite) TestPolicyBadPublish(c *C) {
	for _, key := range []string{"badsize", "badsize2", "badsize3", "nopool", "badsnaps"} {
		c.Assert(s.tlc.PublishPolicy("test", testPolicyConfigs[key]), NotNil)
	}
}

func (s *configSuite) TestPolicyPublishEtcdDown(c *C) {
	stopStartEtcd(c, func() {
		for _, key := range []string{"basic", "basic2"} {
			c.Assert(s.tlc.PublishPolicy("test", testPolicyConfigs[key]), NotNil)
		}
	})
}

func (s *configSuite) TestPolicyListEtcdDown(c *C) {
	stopStartEtcd(c, func() {
		_, err := s.tlc.ListPolicies()
		c.Assert(err, NotNil)
	})
}
