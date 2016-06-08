package config

import (
	"path"
	"sort"

	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/watch"

	. "gopkg.in/check.v1"
)

func (s *configSuite) TestActualSize(c *C) {
	vo := &CreateOptions{Size: "10MB"}
	actualSize, err := vo.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(int(actualSize), Equals, 10)

	vo = &CreateOptions{Size: "1GB"}
	actualSize, err = vo.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(int(actualSize), Equals, 1024)

	vo = &CreateOptions{Size: "0"}
	actualSize, err = vo.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(int(actualSize), Equals, 0)

	vo = &CreateOptions{Size: "10M"}
	_, err = vo.ActualSize()
	c.Assert(err, NotNil)

	vo = &CreateOptions{Size: "garbage"}
	_, err = vo.ActualSize()
	c.Assert(err, NotNil)
}

func (s *configSuite) TestVolumeValidate(c *C) {
	vc := &Volume{
		VolumeName: "foo",
		PolicyName: "policy1",
	}
	c.Assert(vc.Validate(), NotNil)

	vc = &Volume{
		DriverOptions:  map[string]string{"pool": "rbd"},
		CreateOptions:  CreateOptions{Size: "10MB", actualSize: 10},
		RuntimeOptions: RuntimeOptions{UseSnapshots: false},
		VolumeName:     "",
		PolicyName:     "policy1",
	}

	c.Assert(vc.Validate(), NotNil)

	vc = &Volume{
		DriverOptions:  map[string]string{"pool": "rbd"},
		CreateOptions:  CreateOptions{Size: "10MB", actualSize: 10},
		RuntimeOptions: RuntimeOptions{UseSnapshots: false},
		VolumeName:     "foo",
		PolicyName:     "",
	}

	c.Assert(vc.Validate(), NotNil)

	vc = &Volume{
		Backends: BackendDrivers{
			Mount:    "ceph",
			Snapshot: "ceph",
			CRUD:     "ceph",
		},
		DriverOptions:  map[string]string{"pool": "rbd"},
		CreateOptions:  CreateOptions{Size: "10MB", actualSize: 10},
		RuntimeOptions: RuntimeOptions{UseSnapshots: false},
		VolumeName:     "foo",
		PolicyName:     "policy1",
	}

	c.Assert(vc.Validate(), IsNil)
}

func (s *configSuite) TestVolumeOptionsValidate(c *C) {
	opts := CreateOptions{}
	c.Assert(opts.Validate(), IsNil)
	opts2 := RuntimeOptions{}
	c.Assert(opts2.Validate(), IsNil)

	opts = CreateOptions{Size: "0M"}
	c.Assert(opts.Validate(), NotNil)
	opts = CreateOptions{Size: "10MB", actualSize: 10}
	c.Assert(opts.Validate(), IsNil)

	opts2 = RuntimeOptions{UseSnapshots: true}
	c.Assert(opts2.Validate(), NotNil)
	opts2 = RuntimeOptions{UseSnapshots: true, Snapshot: SnapshotConfig{}}
	c.Assert(opts2.Validate(), NotNil)
	opts2 = RuntimeOptions{UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "10m", Keep: 0}}
	c.Assert(opts2.Validate(), NotNil)
	opts2 = RuntimeOptions{UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "", Keep: 10}}
	c.Assert(opts2.Validate(), NotNil)
	opts2 = RuntimeOptions{UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "10m", Keep: 10}}
	c.Assert(opts2.Validate(), IsNil)
}

func (s *configSuite) TestWatchVolumes(c *C) {
	c.Assert(s.tlc.PublishPolicy("policy1", testPolicies["basic"]), IsNil)
	volumeChan := make(chan *watch.Watch)
	s.tlc.WatchVolumeRuntimes(volumeChan)

	vol, err := s.tlc.CreateVolume(Request{Policy: "policy1", Volume: "test"})
	c.Assert(err, IsNil)
	c.Assert(s.tlc.PublishVolume(vol), IsNil)
	vol2 := <-volumeChan
	c.Assert(vol2.Key, Equals, "policy1/test")
	c.Assert(vol2.Config, NotNil)
	volConfig := vol2.Config.(*RuntimeOptions)
	c.Assert(vol.RuntimeOptions, DeepEquals, *volConfig)
}

func (s *configSuite) TestVolumeCRUD(c *C) {
	policyNames := []string{"foo", "bar"}
	volumeNames := []string{"baz", "quux"}
	sort.Strings(volumeNames) // lazy

	_, err := s.tlc.CreateVolume(Request{})
	c.Assert(err, NotNil)

	_, err = s.tlc.CreateVolume(Request{Policy: "Doesn'tExist"})
	c.Assert(err, NotNil)

	// populate the policies so the next few tests don't give false positives
	for _, policy := range policyNames {
		c.Assert(s.tlc.PublishPolicy(policy, testPolicies["basic"]), IsNil)
	}

	_, err = s.tlc.CreateVolume(Request{Policy: "foo", Volume: "bar", Options: map[string]string{"quux": "derp"}})
	c.Assert(err, NotNil)

	_, err = s.tlc.CreateVolume(Request{Policy: "foo", Volume: ""})
	c.Assert(err, NotNil)

	_, err = s.tlc.GetVolume("foo", "bar")
	c.Assert(err, NotNil)

	_, err = s.tlc.ListVolumes("quux")
	c.Assert(err, NotNil)

	for _, policy := range policyNames {
		for _, volume := range volumeNames {
			vcfg, err := s.tlc.CreateVolume(Request{Policy: policy, Volume: volume, Options: map[string]string{"filesystem": ""}})
			c.Assert(err, IsNil)
			c.Assert(s.tlc.PublishVolume(vcfg), IsNil)
			c.Assert(s.tlc.PublishVolume(vcfg), Equals, errors.Exists)

			c.Assert(vcfg.CreateOptions.FileSystem, Equals, "ext4")

			defer func(policy, volume string) { c.Assert(s.tlc.RemoveVolume(policy, volume), IsNil) }(policy, volume)

			c.Assert(vcfg.VolumeName, Equals, volume)

			vcfg2, err := s.tlc.GetVolume(policy, volume)
			c.Assert(err, IsNil)
			c.Assert(vcfg, DeepEquals, vcfg2)

			runtime, err := s.tlc.GetVolumeRuntime(policy, volume)
			c.Assert(err, IsNil)
			c.Assert(runtime, DeepEquals, vcfg.RuntimeOptions)

			vcfg.CreateOptions.Size = "0"
			vcfg.CreateOptions.actualSize = 0
			c.Assert(s.tlc.PublishVolume(vcfg), NotNil)
		}

		volumes, err := s.tlc.ListVolumes(policy)
		c.Assert(err, IsNil)

		volumeKeys := []string{}

		for key := range volumes {
			volumeKeys = append(volumeKeys, key)
		}

		sort.Strings(volumeKeys)

		c.Assert(volumeNames, DeepEquals, volumeKeys)
		for _, vol := range volumes {
			c.Assert(vol.CreateOptions, DeepEquals, testPolicies["basic"].CreateOptions)
			c.Assert(vol.RuntimeOptions, DeepEquals, testPolicies["basic"].RuntimeOptions)
		}
	}

	allVols, err := s.tlc.ListAllVolumes()
	c.Assert(err, IsNil)

	sort.Strings(allVols)

	allNames := []string{}

	for _, policy := range policyNames {
		for _, volume := range volumeNames {
			allNames = append(allNames, path.Join(policy, volume))
		}
	}

	sort.Strings(allNames)

	c.Assert(allNames, DeepEquals, allVols)
}

func (s *configSuite) TestVolumeRuntime(c *C) {
	c.Assert(s.tlc.PublishPolicy("policy1", testPolicies["basic"]), IsNil)
	vol, err := s.tlc.CreateVolume(Request{Policy: "policy1", Volume: "test"})
	c.Assert(err, IsNil)
	c.Assert(s.tlc.PublishVolume(vol), IsNil)
	runtime := vol.RuntimeOptions
	runtime.RateLimit.ReadIOPS = 1000
	c.Assert(s.tlc.PublishVolumeRuntime(vol, runtime), IsNil)

	runtime2, err := s.tlc.GetVolumeRuntime("policy1", "test")
	c.Assert(err, IsNil)
	c.Assert(runtime2.RateLimit.ReadIOPS, Equals, uint(1000))
	c.Assert(runtime, DeepEquals, runtime2)

	vol, err = s.tlc.GetVolume("policy1", "test")
	c.Assert(err, IsNil)
	c.Assert(vol.RuntimeOptions, DeepEquals, runtime2)
	c.Assert(vol.RuntimeOptions.RateLimit.ReadIOPS, Equals, uint(1000))
}

func (s *configSuite) TestToDriverOptions(c *C) {
	c.Assert(s.tlc.PublishPolicy("policy1", testPolicies["basic"]), IsNil)
	vol, err := s.tlc.CreateVolume(Request{Policy: "policy1", Volume: "test"})
	c.Assert(err, IsNil)

	do, err := vol.ToDriverOptions(1)
	c.Assert(err, IsNil)

	expected := storage.DriverOptions{
		Volume: storage.Volume{
			Name:   "policy1/test",
			Size:   0xa,
			Params: storage.Params{"pool": "rbd"},
		},
		FSOptions: storage.FSOptions{
			Type:          "ext4",
			CreateCommand: "",
		},
		Timeout: 1,
		Options: nil,
	}

	c.Assert(do, DeepEquals, expected)
}
