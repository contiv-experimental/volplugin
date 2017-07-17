package config

import (
	"github.com/contiv/volplugin/storage"
	. "gopkg.in/check.v1"
)

var testPolicies = map[string]*Policy{
	"basic": {
		Name: "basic",
		Backends: &BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: storage.DriverParams{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: RuntimeOptions{
			UseSnapshots: true,
			Snapshot: SnapshotConfig{
				Keep:      10,
				Frequency: "1m",
			},
		},
		FileSystems: defaultFilesystems,
	},
	"basic2": {
		Name: "basic2",
		Backends: &BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: storage.DriverParams{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "20MB",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"untouchedwithzerosize": {
		Name: "untouchedwithzerosize",
		Backends: &BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: storage.DriverParams{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "0",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"nilfs": {
		Name: "nilfs",
		Backends: &BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: storage.DriverParams{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "20MB",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsize3": {
		Name: "badsize3",
		Backends: &BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: storage.DriverParams{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "not a number",
			FileSystem: defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsnaps": {
		Name: "badsnaps",
		Backends: &BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: storage.DriverParams{"pool": "rbd"},
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
	"blanksize": {
		Backends: &BackendDrivers{
			Mount: "ceph",
		},
		Name:          "blanksize",
		DriverOptions: storage.DriverParams{"pool": "rbd"},
		CreateOptions: CreateOptions{
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: RuntimeOptions{},
		FileSystems:    defaultFilesystems,
	},
	"blanksizewithcrud": {
		Backends: &BackendDrivers{
			CRUD:  "ceph",
			Mount: "ceph",
		},
		Name:          "blanksize",
		DriverOptions: storage.DriverParams{"pool": "rbd"},
		CreateOptions: CreateOptions{
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: RuntimeOptions{},
		FileSystems:    defaultFilesystems,
	},
	"nobackend": {
		Name:          "nobackend",
		DriverOptions: storage.DriverParams{"pool": "rbd"},
		CreateOptions: CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: RuntimeOptions{},
		FileSystems:    defaultFilesystems,
	},
	"nfs": {
		Backends: &BackendDrivers{
			Mount: "nfs",
		},
	},
	"cephbackend": {
		Backend: "ceph",
		CreateOptions: CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
	},
	"nfsbackend": {
		Backend: "nfs",
		CreateOptions: CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
	},
	"emptybackend": {
		Backend: "",
	},
	"badbackend": {
		Backend: "dummy",
	},
	"backends": { // "Backend" attribute will be ignored
		Backends: &BackendDrivers{
			Mount: "nfs",
		},
		Backend: "ceph",
		CreateOptions: CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
	},
	"badbackends": {
		Backend: "nfs",
		Backends: &BackendDrivers{
			Mount: "", // This should not be empty
		},
	},
}

func (s *configSuite) TestBackendPolicy(c *C) {
	c.Assert(s.tlc.PublishPolicy("cephbackend", testPolicies["cephbackend"]), IsNil)
	c.Assert(s.tlc.PublishPolicy("nfsbackend", testPolicies["nfsbackend"]), IsNil)
	c.Assert(s.tlc.PublishPolicy("backends", testPolicies["backends"]), IsNil)

	c.Assert(s.tlc.PublishPolicy("emptybackend", testPolicies["emptybackend"]), NotNil)
	c.Assert(s.tlc.PublishPolicy("badbackends", testPolicies["badbackends"]), NotNil)
	c.Assert(s.tlc.PublishPolicy("badbackend", testPolicies["badbackend"]), NotNil)
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
			if policy.Name == name {
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
	for _, key := range []string{"basic", "basic2", "nilfs", "blanksize"} {
		c.Assert(testPolicies[key].Validate(), IsNil)
	}

	// FIXME: ensure the default filesystem option is set when validate is called.
	//        honestly, this both a pretty lousy way to do it and test it, we should do
	//        something better.
	c.Assert(testPolicies["nilfs"].FileSystems, DeepEquals, map[string]string{defaultFilesystem: "mkfs.ext4 -m0 %"})

	c.Assert(testPolicies["nobackend"].Validate(), NotNil)
	c.Assert(testPolicies["untouchedwithzerosize"].Validate(), NotNil)
	_, err := testPolicies["badsize3"].CreateOptions.ActualSize()
	c.Assert(err, NotNil)
}

func (s *configSuite) TestPolicyBadPublish(c *C) {
	for _, key := range []string{"nobackend", "badsize3", "badsnaps", "blanksizewithcrud"} {
		c.Assert(s.tlc.PublishPolicy("test", testPolicies[key]), NotNil, Commentf(key))
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
