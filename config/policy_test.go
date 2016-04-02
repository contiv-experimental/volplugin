package config

import . "gopkg.in/check.v1"

var testPolicies = map[string]*Policy{
	"basic": {
		Backend:       "ceph",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"basic2": {
		Backend:       "ceph",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "20MB",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"untouchedwithzerosize": {
		Backend:       "ceph",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "0",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"nilfs": {
		Backend:       "ceph",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "20MB",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsize": {
		Backend:       "ceph",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "0",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsize2": {
		Backend:       "ceph",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "10M",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsize3": {
		Backend:       "ceph",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "not a number",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsnaps": {
		Backend:       "ceph",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: RuntimeOptions{
			UseSnapshots: true,
			Snapshot: SnapshotConfig{
				Keep:      0,
				Frequency: "",
			},
		},
		FileSystems: defaultFilesystems,
	},
	"nobackend": {
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: RuntimeOptions{
			UseSnapshots: true,
			Snapshot: SnapshotConfig{
				Keep:      0,
				Frequency: "",
			},
		},
		FileSystems: defaultFilesystems,
	},
}

func (s *configSuite) TestBasicPolicy(c *C) {
	c.Assert(s.tlc.PublishPolicy("quux", testPolicies["basic"]), IsNil)

	cfg, err := s.tlc.GetPolicy("quux")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testPolicies["basic"])

	c.Assert(s.tlc.PublishPolicy("bar", testPolicies["basic2"]), IsNil)

	cfg, err = s.tlc.GetPolicy("bar")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testPolicies["basic2"])

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
	c.Assert(cfg, DeepEquals, testPolicies["basic"])
}

func (s *configSuite) TestPolicyValidate(c *C) {
	for _, key := range []string{"basic", "basic2", "nilfs"} {
		c.Assert(testPolicies[key].Validate(), IsNil)
	}

	// FIXME: ensure the default filesystem option is set when validate is called.
	//        honestly, this both a pretty lousy way to do it and test it, we should do
	//        something better.
	c.Assert(testPolicies["nilfs"].FileSystems, DeepEquals, map[string]string{defaultFilesystem: "mkfs.ext4 -m0 %"})

	c.Assert(testPolicies["nobackend"].Validate(), NotNil)
	c.Assert(testPolicies["untouchedwithzerosize"].Validate(), NotNil)
	c.Assert(testPolicies["badsize"].Validate(), NotNil)
	c.Assert(testPolicies["badsize2"].Validate(), NotNil)
	_, err := testPolicies["badsize3"].CreateOptions.ActualSize()
	c.Assert(err, NotNil)
}

func (s *configSuite) TestPolicyBadPublish(c *C) {
	for _, key := range []string{"nobackend", "badsize", "badsize2", "badsize3", "badsnaps"} {
		c.Assert(s.tlc.PublishPolicy("test", testPolicies[key]), NotNil)
	}
}

func (s *configSuite) TestPolicyPublishEtcdDown(c *C) {
	stopStartEtcd(c, func() {
		for _, key := range []string{"basic", "basic2"} {
			c.Assert(s.tlc.PublishPolicy("test", testPolicies[key]), NotNil)
		}
	})
}

func (s *configSuite) TestPolicyListEtcdDown(c *C) {
	stopStartEtcd(c, func() {
		_, err := s.tlc.ListPolicies()
		c.Assert(err, NotNil)
	})
}
