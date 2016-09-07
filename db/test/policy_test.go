package test

import (
	"github.com/contiv/volplugin/db"
	. "gopkg.in/check.v1"
)

var testPolicies = map[string]*db.Policy{
	"basic": {
		Name: "basic",
		Backends: &db.BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: db.CreateOptions{
			Size:       "10MB",
			FileSystem: db.DefaultFilesystem,
		},
		RuntimeOptions: &db.RuntimeOptions{
			UseSnapshots: true,
			Snapshot: db.SnapshotConfig{
				Keep:      10,
				Frequency: "1m",
			},
		},
		FileSystems: db.DefaultFilesystems,
	},
	"basic2": {
		Name: "basic2",
		Backends: &db.BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: db.CreateOptions{
			Size:       "20MB",
			FileSystem: db.DefaultFilesystem,
		},
		RuntimeOptions: &db.RuntimeOptions{},
		FileSystems:    db.DefaultFilesystems,
	},
	"untouchedwithzerosize": {
		Name: "untouchedwithzerosize",
		Backends: &db.BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: db.CreateOptions{
			Size:       "0",
			FileSystem: db.DefaultFilesystem,
		},
		FileSystems: db.DefaultFilesystems,
	},
	"badsize3": {
		Name: "badsize3",
		Backends: &db.BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: db.CreateOptions{
			Size:       "not a number",
			FileSystem: db.DefaultFilesystem,
		},
		FileSystems: db.DefaultFilesystems,
	},
	"badsnaps": {
		Name: "badsnaps",
		Backends: &db.BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: db.CreateOptions{
			Size:       "10MB",
			FileSystem: db.DefaultFilesystem,
		},
		RuntimeOptions: &db.RuntimeOptions{
			UseSnapshots: true,
			Snapshot: db.SnapshotConfig{
				Keep:      0,
				Frequency: "",
			},
		},
		FileSystems: db.DefaultFilesystems,
	},
	"blanksize": {
		Backends: &db.BackendDrivers{
			Mount: "ceph",
		},
		Name:          "blanksize",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: db.CreateOptions{
			FileSystem: db.DefaultFilesystem,
		},
		RuntimeOptions: &db.RuntimeOptions{},
		FileSystems:    db.DefaultFilesystems,
	},
	"blanksizewithcrud": {
		Backends: &db.BackendDrivers{
			CRUD:  "ceph",
			Mount: "ceph",
		},
		Name:          "blanksize",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: db.CreateOptions{
			FileSystem: db.DefaultFilesystem,
		},
		RuntimeOptions: &db.RuntimeOptions{},
		FileSystems:    db.DefaultFilesystems,
	},
	"nobackend": {
		Name:          "nobackend",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: db.CreateOptions{
			Size:       "10MB",
			FileSystem: db.DefaultFilesystem,
		},
		RuntimeOptions: &db.RuntimeOptions{},
		FileSystems:    db.DefaultFilesystems,
	},
	"nfs": {
		Name: "nfs",
		Backends: &db.BackendDrivers{
			Mount: "nfs",
		},
		CreateOptions:  db.CreateOptions{},
		RuntimeOptions: &db.RuntimeOptions{},
	},
	"cephbackend": {
		Name:    "cephbackend",
		Backend: "ceph",
		CreateOptions: db.CreateOptions{
			Size:       "10MB",
			FileSystem: db.DefaultFilesystem,
		},
	},
	"nfsbackend": {
		Name:    "nfsbackend",
		Backend: "nfs",
		CreateOptions: db.CreateOptions{
			Size:       "10MB",
			FileSystem: db.DefaultFilesystem,
		},
		RuntimeOptions: &db.RuntimeOptions{},
	},
	"emptybackend": {
		Name:    "emptybackend",
		Backend: "",
	},
	"badbackend": {
		Name:    "badbackend",
		Backend: "dummy",
	},
	"backends": { // "Backend" attribute will be ignored
		Name: "backends",
		Backends: &db.BackendDrivers{
			Mount: "nfs",
		},
		Backend: "ceph",
		CreateOptions: db.CreateOptions{
			Size:       "10MB",
			FileSystem: db.DefaultFilesystem,
		},
	},
	"badbackends": {
		Name:    "badbackends",
		Backend: "nfs",
		Backends: &db.BackendDrivers{
			Mount: "", // This should not be empty
		},
	},
}

var (
	passingPolicies = []string{
		"basic",
		"basic2",
		"nfsbackend",
		"nfs",
		"blanksize",
	}

	failingPolicies = []string{
		"untouchedwithzerosize",
		"badsize3",
		"badsnaps",
		"blanksizewithcrud",
		"nobackend",
		"emptybackend",
		"badbackend",
		"backends",
		"badbackends",
	}
)

func (s *testSuite) TestPolicyCRUD(c *C) {
	for _, name := range passingPolicies {
		c.Assert(s.client.Set(testPolicies[name]), IsNil, Commentf("%v", name))
		policy := db.NewPolicy(testPolicies[name].Name)
		c.Assert(s.client.Get(policy), IsNil, Commentf("%v", name))
		c.Assert(policy, DeepEquals, testPolicies[name])
	}

	for _, name := range failingPolicies {
		c.Assert(s.client.Set(testPolicies[name]), NotNil, Commentf("%v", name))
	}
}

func (s *testSuite) TestCopy(c *C) {
	policyCopy := testPolicies["basic"].Copy()
	policyCopy.(*db.Policy).RuntimeOptions.UseSnapshots = false
	c.Assert(testPolicies["basic"].RuntimeOptions.UseSnapshots, Equals, true, Commentf("runtime options pointer was not copied"))
}
