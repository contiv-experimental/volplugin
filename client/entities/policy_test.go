package entities

import (
	. "gopkg.in/check.v1"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/client"
	"github.com/contiv/volplugin/errors"
)

var testPolicies = map[string]*Policy{
	"basic": {
		Name: "basic",
		Backends: &BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: &CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: &RuntimeOptions{
			UseSnapshots: true,
			Snapshot: &SnapshotConfig{
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
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: &CreateOptions{
			Size:       "20MB",
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: &RuntimeOptions{},
		FileSystems:    defaultFilesystems,
	},
	"untouchedwithzerosize": {
		Name: "untouchedwithzerosize",
		Backends: &BackendDrivers{
			CRUD:     "ceph",
			Mount:    "ceph",
			Snapshot: "ceph",
		},
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: &CreateOptions{
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
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: &CreateOptions{
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
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: &CreateOptions{
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
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: &CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: &RuntimeOptions{
			UseSnapshots: true,
			Snapshot: &SnapshotConfig{
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
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: &CreateOptions{
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: &RuntimeOptions{},
		FileSystems:    defaultFilesystems,
	},
	"blanksizewithcrud": {
		Backends: &BackendDrivers{
			CRUD:  "ceph",
			Mount: "ceph",
		},
		Name:          "blanksize",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: &CreateOptions{
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: &RuntimeOptions{},
		FileSystems:    defaultFilesystems,
	},
	"nobackend": {
		Name:          "nobackend",
		DriverOptions: map[string]string{"pool": "rbd"},
		CreateOptions: &CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: &RuntimeOptions{},
		FileSystems:    defaultFilesystems,
	},
	"nfs": {
		Name: "nfs",
		Backends: &BackendDrivers{
			Mount: "nfs",
		},
		CreateOptions:  &CreateOptions{},
		RuntimeOptions: &RuntimeOptions{},
	},
	"cephbackend": {
		Name:    "cephbackend",
		Backend: "ceph",
		CreateOptions: &CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
	},
	"nfsbackend": {
		Name:    "nfsbackend",
		Backend: "nfs",
		CreateOptions: &CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
		RuntimeOptions: &RuntimeOptions{},
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
		Backends: &BackendDrivers{
			Mount: "nfs",
		},
		Backend: "ceph",
		CreateOptions: &CreateOptions{
			Size:       "10MB",
			FileSystem: defaultFilesystem,
		},
	},
	"badbackends": {
		Name:    "badbackends",
		Backend: "nfs",
		Backends: &BackendDrivers{
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
		"nilfs",
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

// This is in test and not exported because it should not be used outside of
// tests. validation goes through the JSON interfaces instead. This is as basic
// as a policy gets.
func newPolicy(name string) *Policy {
	return &Policy{
		Name:           name,
		Backend:        "ceph",
		RuntimeOptions: &RuntimeOptions{},
		CreateOptions:  &CreateOptions{Size: "10GB"},
	}
}

func (s *entitySuite) TestPolicyPath(c *C) {
	policy := newPolicy("test")
	c.Assert(policy.Name, Equals, "test")
	path, err := policy.Path(client.NewEtcdPather("volplugin"))
	c.Assert(err, IsNil)
	c.Assert(path, Equals, "/volplugin/policies/test")

	policy.Name = "" // no name should be a failure.
	path, err = policy.Path(client.NewEtcdPather("volplugin"))

	c.Assert(err, NotNil)
	er, ok := err.(*errored.Error)
	c.Assert(ok, Equals, true)
	c.Assert(er.Contains(errors.InvalidPolicy), Equals, true)
	c.Assert(path, Equals, "")
}

func (s *entitySuite) TestPolicyCRUD(c *C) {
	pc := NewPolicyClient(s.client)

	for _, passing := range passingPolicies {
		err := pc.Publish(testPolicies[passing])
		c.Assert(err, IsNil, Commentf("%v %v", passing, err))
		policy, err := pc.Get(passing)
		c.Assert(err, IsNil, Commentf("%v %v", passing, err))
		c.Assert(policy, DeepEquals, testPolicies[passing], Commentf("%v", passing))
	}

	policies, err := pc.Collection().List()
	c.Assert(err, IsNil)
	c.Assert(len(policies), Not(Equals), 0)

	for _, policy := range policies {
		c.Assert(policy, DeepEquals, testPolicies[policy.Name])
	}

	for _, passing := range passingPolicies {
		err = pc.Delete(passing)
		c.Assert(err, IsNil, Commentf("%v %v", passing, err))
		_, err = pc.Get(passing)
		c.Assert(err, NotNil, Commentf("%v %v", passing, err))
	}

	for _, failing := range failingPolicies {
		c.Assert(pc.Publish(testPolicies[failing]), NotNil, Commentf("%v", failing))
	}

	// policies automatically get added to the history when they are published.
	// this queries the basic policy's history.
	phc := NewPolicyHistoryClient(pc)
	archives, err := phc.Collection().List(testPolicies["basic"])
	c.Assert(err, IsNil)
	c.Assert(len(archives), Equals, 1)
	for key, value := range archives { // there's only one in here, but we don't know the name
		c.Assert(value, DeepEquals, testPolicies["basic"])
		policy, err := phc.Get("basic", key)
		c.Assert(err, IsNil)
		c.Assert(policy, DeepEquals, testPolicies["basic"])
	}
}

func (s *entitySuite) TestPolicyWatch(c *C) {
	pc := NewPolicyClient(s.client)
	channel, errChan, err := pc.Watch("basic")
	c.Assert(err, IsNil)
	payload, err := testPolicies["basic"].Payload()
	c.Assert(err, IsNil)
	path, err := pc.prefix.Replace("policies", "basic")
	c.Assert(err, IsNil)
	s.setKey(path, payload)

	select {
	case err := <-errChan:
		c.Assert(err, IsNil) // this should fail
		c.Fail()             // should never make it here, but if we get a nil error on the channel, that's a real failure.
	case policy := <-channel:
		c.Assert(policy, DeepEquals, testPolicies["basic"])
	}

	channel, errChan = pc.Collection().WatchAll()
	for _, policy := range passingPolicies {
		payload, err := testPolicies[policy].Payload()
		c.Assert(err, IsNil, Commentf("%v", policy))
		path, err := pc.prefix.Replace("policies", "basic")
		c.Assert(err, IsNil)
		s.setKey(path, payload)
	}

	for i := 0; i < len(passingPolicies); i++ {
		select {
		case err := <-errChan:
			c.Assert(err, IsNil) // this should fail
			c.Fail()             // should never make it here, but if we get a nil error on the channel, that's a real failure.
		case policy := <-channel:
			c.Assert(policy, DeepEquals, testPolicies[policy.(*Policy).Name], Commentf("%v", policy))
		}
	}

	phc := NewPolicyHistoryClient(pc)
	channel, errChan = phc.Collection().WatchAll()
	c.Assert(pc.Publish(testPolicies["basic"]), IsNil)
	select {
	case err := <-errChan:
		c.Assert(err, IsNil) // this should fail
		c.Fail()             // should never make it here, but if we get a nil error on the channel, that's a real failure.
	case policy := <-channel:
		c.Assert(policy, DeepEquals, testPolicies["basic"])
	}
}
