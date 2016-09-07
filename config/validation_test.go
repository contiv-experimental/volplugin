package config

import (
	"github.com/contiv/volplugin/storage"
	. "gopkg.in/check.v1"
)

var (
	defaultBackends = &BackendDrivers{CRUD: "ceph", Mount: "ceph", Snapshot: "ceph"}
	VolumeConfigs   = map[string]map[string]*Volume{
		"valid": {
			"basic": {
				DriverOptions:  storage.DriverParams{"pool": "rbd"},
				CreateOptions:  CreateOptions{Size: "10MB"},
				RuntimeOptions: RuntimeOptions{UseSnapshots: false},
				VolumeName:     "basicvolume",
				PolicyName:     "basicpolicy",
				Backends:       defaultBackends,
			},
			"basicwithruntime": {
				DriverOptions:  storage.DriverParams{"pool": "rbd"},
				CreateOptions:  CreateOptions{Size: "10MB"},
				RuntimeOptions: RuntimeOptions{UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "10m", Keep: 10}},
				VolumeName:     "basicvolume",
				PolicyName:     "basicpolicy",
				Backends:       defaultBackends,
			},
		},
		"invalid": {
			"emptyvolume": {}, // base case
			"emptyname": {
				VolumeName:     "",
				PolicyName:     "policy",
				Backends:       defaultBackends,
				RuntimeOptions: RuntimeOptions{UseSnapshots: false},
			},
			"emptypolicy": {
				VolumeName:     "volume",
				PolicyName:     "",
				Backends:       defaultBackends,
				RuntimeOptions: RuntimeOptions{UseSnapshots: false},
			},
			"emptybackends": {
				VolumeName:     "volume",
				PolicyName:     "policy",
				RuntimeOptions: RuntimeOptions{UseSnapshots: false},
			},
			"invalidnames": {
				VolumeName:     "volume/1.2",
				PolicyName:     "policy/1/2",
				RuntimeOptions: RuntimeOptions{UseSnapshots: false},
			},
			"invalidvolumename": {
				VolumeName:     "volume.1",
				PolicyName:     "policy",
				RuntimeOptions: RuntimeOptions{UseSnapshots: false},
			},
			"invalidpolicyname": {
				VolumeName:     "volume",
				PolicyName:     "policy.1/2",
				RuntimeOptions: RuntimeOptions{UseSnapshots: false},
			},
		},
	}
	PolicyConfigs = map[string]map[string]*Policy{
		"valid": {
			"basic": testPolicies["basic"],
			"basicceph": {
				Name: "basicceph",
				Backends: &BackendDrivers{
					CRUD:     "ceph",
					Mount:    "ceph",
					Snapshot: "ceph",
				},
			},
			"basicnfs": {
				Name: "basicnfs",
				Backends: &BackendDrivers{
					CRUD:     "",
					Mount:    "nfs",
					Snapshot: "",
				},
			},
			"backendceph": {
				Name:    "backendceph",
				Backend: "ceph",
			},
			"backendnfs": {
				Name:    "backendnfs",
				Backend: "nfs",
			},
			"withbackendattr1": {
				Name: "withbackendattr1",
				Backends: &BackendDrivers{
					CRUD:     "ceph",
					Mount:    "ceph",
					Snapshot: "ceph",
				},
				Backend: "nfs",
			},
			"withbackendattr2": {
				Name: "withbackendattr2",
				Backends: &BackendDrivers{
					CRUD:     "",
					Mount:    "nfs",
					Snapshot: "",
				},
				Backend: "ceph",
			},
		},
		"invalid": {
			"emptyname": {
				Name:    "",
				Backend: "ceph",
			},
			"badbackend": {
				Name:    "badbackend",
				Backend: "bad",
			},
			"emptybackendconfig": {
				Name: "emptybackendconfig",
			},
			"emptybackend": {
				Name:    "emptybackend",
				Backend: "", // omitempty
			},
			"emptybackendsconfig": {
				Name: "emptybackendsconfig",
				Backends: &BackendDrivers{ // this field will be omitted - "omitempty"
					CRUD:     "",
					Mount:    "",
					Snapshot: "",
				},
			},
			"badbackendsconfig": {
				Name: "badbackendsconfig",
				Backends: &BackendDrivers{
					CRUD:     "bad",
					Mount:    "bad",
					Snapshot: "bad",
				},
			},
			"nomount": {
				Name:     "nomount",
				Backends: &BackendDrivers{},
			},
			"nfsbackends": { //only "nfs" mount it is valid
				Name: "crudnfs",
				Backends: &BackendDrivers{
					CRUD:     "nfs",
					Mount:    "nfs",
					Snapshot: "nfs",
				},
			},
			"invalidpolicyname1": {
				Name:    "invalidpolicyname.1",
				Backend: "ceph",
			},
			"invalidpolicyname2": {
				Name:    "invalidpolicyname/1/2",
				Backend: "ceph",
			},
		},
	}
	RuntimeConfigs = map[string]map[string]*RuntimeOptions{
		"valid": {
			"snapshots": {
				UseSnapshots: true,
				Snapshot: SnapshotConfig{
					Keep:      10,
					Frequency: "1m",
				},
			},
			"nosnapshots": {
				UseSnapshots: false,
			},
		},
		"invalid": {
			"nosnapshotconfig": {
				UseSnapshots: true, // requires snapshot configuration
			},
			"zerokeepvalue": {
				UseSnapshots: true,
				Snapshot: SnapshotConfig{
					Keep:      0, // anything <1 is invalid
					Frequency: "1m",
				},
			},
			"emptyfrequency": {
				UseSnapshots: true,
				Snapshot: SnapshotConfig{
					Keep:      1,
					Frequency: "", // "" not allowed
				},
			},
			"invalidfrequency": {
				UseSnapshots: true,
				Snapshot: SnapshotConfig{
					Frequency: "badvalue", // accepts only (^[0-9]+.$)
					Keep:      1,
				},
			},
			"nofrequency": {
				UseSnapshots: true,
				Snapshot: SnapshotConfig{ // requires frequency
					Keep: 10,
				},
			},
			"nokeep": {
				UseSnapshots: true,
				Snapshot: SnapshotConfig{ // requires keep value
					Frequency: "10d",
				},
			},
			"invalidsnapshotconfig": {
				UseSnapshots: true,
				Snapshot: SnapshotConfig{ // invalid frequency and keep values
					Frequency: "test",
					Keep:      0,
				},
			},
		},
	}
)

func (s *configSuite) TestRuntimeValidation(c *C) {
	for configname, ro := range RuntimeConfigs["valid"] {
		c.Assert(ro.ValidateJSON(), IsNil, Commentf("%s", configname))
	}

	invalidRuntimeConfigs := RuntimeConfigs["invalid"]

	err := invalidRuntimeConfigs["nosnapshotconfig"].ValidateJSON()
	c.Assert(err, ErrorMatches, "(?m)*snapshot.frequency:.*length must be greater than or equal to 1.*")
	c.Assert(err, ErrorMatches, "(?m)*snapshot.keep:.*greater than or equal to 1.*")

	c.Assert(invalidRuntimeConfigs["zerokeepvalue"].ValidateJSON(), ErrorMatches, "(?m)*snapshot.keep:.*greater than or equal to 1.*")

	c.Assert(invalidRuntimeConfigs["emptyfrequency"].ValidateJSON(), ErrorMatches, "(?m)*snapshot.frequency:.*length must be greater than or equal to 1.*")

	c.Assert(invalidRuntimeConfigs["invalidfrequency"].ValidateJSON(), ErrorMatches, "(?m)*snapshot.frequency:.*Does not match pattern.*")

	c.Assert(invalidRuntimeConfigs["nofrequency"].ValidateJSON(), ErrorMatches, "(?m)*snapshot.frequency:.*length must be greater than or equal to 1.*")

	c.Assert(invalidRuntimeConfigs["nokeep"].ValidateJSON(), ErrorMatches, "(?m)*snapshot.keep:.*greater than or equal to 1.*")

	err = invalidRuntimeConfigs["invalidsnapshotconfig"].ValidateJSON()
	c.Assert(err, ErrorMatches, "(?m)*snapshot.frequency:.*Does not match pattern.*")
	c.Assert(err, ErrorMatches, "(?m)*snapshot.keep:.*greater than or equal to 1.*")
}

func (s *configSuite) TestSingletonBackend(c *C) {
	PolicyConfigs["valid"]["backendceph"].Validate() // Policy Validation
	s.validateBackendsConfig(c, PolicyConfigs["valid"]["backendceph"].Backends, "ceph", "ceph", "ceph")

	PolicyConfigs["valid"]["backendnfs"].Validate()
	s.validateBackendsConfig(c, PolicyConfigs["valid"]["backendnfs"].Backends, "", "nfs", "")

	// Below test ensures that "Validate" did not change the given "backends" config, in case there is one provided
	PolicyConfigs["valid"]["basicceph"].Validate()
	s.validateBackendsConfig(c, PolicyConfigs["valid"]["basicceph"].Backends, "ceph", "ceph", "ceph")

	PolicyConfigs["valid"]["basicnfs"].Validate()
	s.validateBackendsConfig(c, PolicyConfigs["valid"]["basicnfs"].Backends, "", "nfs", "")

	// Ensures "Backends" is given priority over "Backend" attribute
	PolicyConfigs["valid"]["withbackendattr1"].Validate()
	s.validateBackendsConfig(c, PolicyConfigs["valid"]["withbackendattr1"].Backends, "ceph", "ceph", "ceph")

	PolicyConfigs["valid"]["withbackendattr2"].Validate()
	s.validateBackendsConfig(c, PolicyConfigs["valid"]["withbackendattr2"].Backends, "", "nfs", "")
}

func (s *configSuite) validateBackendsConfig(c *C, backends *BackendDrivers, crud string, mount string, snapshot string) {
	c.Assert(backends.CRUD, Equals, crud)
	c.Assert(backends.Mount, Equals, mount)
	c.Assert(backends.Snapshot, Equals, snapshot)
}

func (s *configSuite) TestPolicyValidation(c *C) {
	for configname, policy := range PolicyConfigs["valid"] {
		c.Assert(policy.ValidateJSON(), IsNil, Commentf("%s", configname))
	}

	invalidPolicyConfigs := PolicyConfigs["invalid"]

	c.Assert(invalidPolicyConfigs["emptyname"].ValidateJSON(), ErrorMatches, "(?m)*name:.*length must be greater than or equal to 1.*")

	c.Assert(invalidPolicyConfigs["badbackend"].ValidateJSON(), ErrorMatches, "(?m)*backend must be one of.*")

	c.Assert(invalidPolicyConfigs["emptybackendconfig"].ValidateJSON(), ErrorMatches, "(?m)*backend is required.*")
	c.Assert(invalidPolicyConfigs["emptybackend"].ValidateJSON(), ErrorMatches, "(?m)*backend is required.*")

	c.Assert(invalidPolicyConfigs["emptybackendsconfig"].ValidateJSON(), ErrorMatches, "(?m)*backends.mount must be one of.*")

	err := invalidPolicyConfigs["badbackendsconfig"].ValidateJSON()
	c.Assert(err, ErrorMatches, "(?m)*backends.mount must be one.*")
	c.Assert(err, ErrorMatches, "(?m)*backends.crud must be one.*")
	c.Assert(err, ErrorMatches, "(?m)*backends.snapshot must be one.*")

	err = invalidPolicyConfigs["nfsbackends"].ValidateJSON()
	c.Assert(err, ErrorMatches, "(?m)*backends.crud must be one.*")
	c.Assert(err, ErrorMatches, "(?m)*backends.snapshot must be one.*")

	c.Assert(invalidPolicyConfigs["invalidpolicyname1"].ValidateJSON(), ErrorMatches, "(?m)*name: Does not match pattern.*")
	c.Assert(invalidPolicyConfigs["invalidpolicyname2"].ValidateJSON(), ErrorMatches, "(?m)*name: Does not match pattern.*")
}

//XXX: All the possible runtime configs are tested in TestRuntimeValidation, so I'm not repeating it here.
func (s *configSuite) TestVolumeValidation(c *C) {
	for configname, volume := range VolumeConfigs["valid"] {
		c.Assert(volume.ValidateJSON(), IsNil, Commentf("%s", configname))
	}

	invalidVolumeConfigs := VolumeConfigs["invalid"]

	err := invalidVolumeConfigs["emptyvolume"].ValidateJSON()
	c.Assert(err, ErrorMatches, "(?m)*name:.*length must be greater than or equal to 1.*")
	c.Assert(err, ErrorMatches, "(?m)*policy:.*length must be greater than or equal to 1.*")
	c.Assert(err, ErrorMatches, "(?m)*backends is required.*")

	c.Assert(invalidVolumeConfigs["emptyname"].ValidateJSON(), ErrorMatches, "(?m)*name:.*length must be greater than or equal to 1.*")
	c.Assert(invalidVolumeConfigs["emptypolicy"].ValidateJSON(), ErrorMatches, "(?m)*policy:.*length must be greater than or equal to 1.*")
	c.Assert(invalidVolumeConfigs["emptybackends"].ValidateJSON(), ErrorMatches, "(?m)*backends is required.*")

	err = invalidVolumeConfigs["invalidnames"].ValidateJSON()
	c.Assert(err, ErrorMatches, "(?m)*name: Does not match pattern.*")
	c.Assert(err, ErrorMatches, "(?m)*policy: Does not match pattern.*")

	c.Assert(invalidVolumeConfigs["invalidvolumename"].ValidateJSON(), ErrorMatches, "(?m)*name: Does not match pattern.*")
	c.Assert(invalidVolumeConfigs["invalidpolicyname"].ValidateJSON(), ErrorMatches, "(?m)*policy: Does not match pattern.*")
}
