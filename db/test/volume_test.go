package test

import (
	"fmt"
	"sort"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage"
	. "gopkg.in/check.v1"
)

func (s *testSuite) TestVolumeCRUD(c *C) {
	policyNames := []string{"foo", "bar"}
	volumeNames := []string{"baz", "quux"}

	c.Assert(s.client.Set(&db.Volume{}), NotNil)

	_, err := db.CreateVolume(&db.VolumeRequest{Policy: nil})
	c.Assert(err, NotNil)

	// populate the policies so the next few tests don't give false positives
	for _, policy := range policyNames {
		copy := testPolicies["basic"].Copy()
		copy.(*db.Policy).Name = policy
		err := s.client.Set(copy)
		c.Assert(err, IsNil, Commentf("%v", err))
	}

	_, err = db.CreateVolume(&db.VolumeRequest{Policy: db.NewPolicy("foo"), Name: "bar", Options: map[string]string{"quux": "derp"}})
	c.Assert(err, NotNil)

	_, err = db.CreateVolume(&db.VolumeRequest{Policy: db.NewPolicy("foo"), Name: ""})
	c.Assert(err, NotNil)

	vol := db.NewVolume("foo", "bar")
	c.Assert(s.client.Get(vol).(*errored.Error).Contains(errors.NotExists), Equals, true)

	// _, err = s.client.ListVolumes("quux")
	// c.Assert(err, NotNil)

	for _, policyName := range policyNames {
		for _, volumeName := range volumeNames {
			policy := db.NewPolicy(policyName)
			c.Assert(s.client.Get(policy), IsNil)
			vcfg, err := db.CreateVolume(&db.VolumeRequest{Policy: policy, Name: volumeName, Options: map[string]string{"filesystem": ""}})
			c.Assert(err, IsNil)
			err = s.client.Set(vcfg)
			c.Assert(err, IsNil, Commentf("%v", err))
			err = s.client.Set(vcfg)
			c.Assert(err.(*errored.Error).Contains(errors.Exists), Equals, true)

			c.Assert(vcfg.CreateOptions.FileSystem, Equals, "ext4")

			defer func() { c.Assert(s.client.Delete(vcfg), IsNil) }()

			c.Assert(vcfg.VolumeName, Equals, volumeName)

			vcfg2 := db.NewVolume(policyName, volumeName)
			c.Assert(s.client.Get(vcfg2), IsNil)
			c.Assert(vcfg, DeepEquals, vcfg2)

			runtime := db.NewRuntimeOptions(policyName, volumeName)
			c.Assert(s.client.Get(runtime), IsNil)
			c.Assert(runtime, DeepEquals, vcfg.RuntimeOptions)

			vcfg.CreateOptions.Size = "0"
			c.Assert(s.client.Set(vcfg), NotNil)
		}

		volumes, err := s.client.ListPrefix(policyName, &db.Volume{})
		c.Assert(err, IsNil)

		volumeKeys := []string{}
		for _, volume := range volumes {
			volumeKeys = append(volumeKeys, volume.(*db.Volume).VolumeName)
		}

		sort.Strings(volumeKeys)

		c.Assert(volumeNames, DeepEquals, volumeKeys)
		for _, entity := range volumes {
			vol := entity.(*db.Volume)
			testPolicies["basic"].RuntimeOptions.SetKey(vol.String())
			c.Assert(vol.CreateOptions, DeepEquals, testPolicies["basic"].CreateOptions)
			c.Assert(vol.RuntimeOptions, DeepEquals, testPolicies["basic"].RuntimeOptions)
		}
	}

	allVols, err := s.client.List(&db.Volume{})
	c.Assert(err, IsNil)

	for _, policy := range policyNames {
		for _, volume := range volumeNames {
			found := false
			for _, ent := range allVols {
				vol := ent.(*db.Volume)
				if vol.PolicyName == policy && vol.VolumeName == volume {
					found = true
				}

				c.Assert(vol.CreateOptions, DeepEquals, testPolicies["basic"].CreateOptions)
				// Cannot use deepequals because of the private members in runtimeoptions now.
				c.Assert(vol.RuntimeOptions.UseSnapshots, Equals, testPolicies["basic"].RuntimeOptions.UseSnapshots)
				c.Assert(vol.RuntimeOptions.Snapshot, DeepEquals, testPolicies["basic"].RuntimeOptions.Snapshot)
				c.Assert(vol.RuntimeOptions.RateLimit, DeepEquals, testPolicies["basic"].RuntimeOptions.RateLimit)
			}

			c.Assert(found, Equals, true, Commentf("%s/%s", policy, volume))
		}
	}
}

func (s *testSuite) TestActualSize(c *C) {
	vo := &db.CreateOptions{Size: "10MB"}
	actualSize, err := vo.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(int(actualSize), Equals, 10)

	vo = &db.CreateOptions{Size: "1GB"}
	actualSize, err = vo.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(int(actualSize), Equals, 1000)

	vo = &db.CreateOptions{Size: "0"}
	actualSize, err = vo.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(int(actualSize), Equals, 0)

	vo = &db.CreateOptions{Size: "10M"}
	size, err := vo.ActualSize()
	c.Assert(size, Equals, uint64(10))
	c.Assert(err, IsNil)

	vo = &db.CreateOptions{Size: "garbage"}
	_, err = vo.ActualSize()
	c.Assert(err, NotNil)
}

func (s *testSuite) TestVolumeValidate(c *C) {
	vc := &db.Volume{
		VolumeName: "foo",
		PolicyName: "policy1",
	}
	c.Assert(vc.Validate(), NotNil)

	vc = &db.Volume{
		DriverOptions:  map[string]string{"pool": "rbd"},
		CreateOptions:  db.CreateOptions{Size: "10MB"},
		RuntimeOptions: &db.RuntimeOptions{UseSnapshots: false},
		VolumeName:     "",
		PolicyName:     "policy1",
	}

	c.Assert(vc.Validate(), NotNil)

	vc = &db.Volume{
		DriverOptions:  map[string]string{"pool": "rbd"},
		CreateOptions:  db.CreateOptions{Size: "10MB"},
		RuntimeOptions: &db.RuntimeOptions{UseSnapshots: false},
		VolumeName:     "foo",
		PolicyName:     "",
	}

	c.Assert(vc.Validate(), NotNil)

	vc = &db.Volume{
		Backends: &db.BackendDrivers{
			Mount:    "ceph",
			Snapshot: "ceph",
			CRUD:     "ceph",
		},
		DriverOptions:  map[string]string{"pool": "rbd"},
		CreateOptions:  db.CreateOptions{Size: "10MB"},
		RuntimeOptions: &db.RuntimeOptions{UseSnapshots: false},
		VolumeName:     "foo",
		PolicyName:     "policy1",
	}

	c.Assert(vc.Validate(), IsNil)
}

func (s *testSuite) TestVolumeOptionsValidate(c *C) {
	opts := &db.RuntimeOptions{UseSnapshots: true}
	c.Assert(opts.Validate(), NotNil)
	opts = &db.RuntimeOptions{UseSnapshots: true, Snapshot: db.SnapshotConfig{Frequency: "10m", Keep: 0}}
	c.Assert(opts.Validate(), NotNil)
	opts = &db.RuntimeOptions{UseSnapshots: true, Snapshot: db.SnapshotConfig{Frequency: "", Keep: 10}}
	c.Assert(opts.Validate(), NotNil)
	opts = &db.RuntimeOptions{UseSnapshots: true, Snapshot: db.SnapshotConfig{Frequency: "10m", Keep: 10}}
	c.Assert(opts.Validate(), IsNil)
}

func (s *testSuite) TestToDriverOptions(c *C) {
	c.Assert(s.client.Set(testPolicies["basic"]), IsNil)
	vol, err := db.CreateVolume(&db.VolumeRequest{Policy: testPolicies["basic"], Name: "test"})
	c.Assert(err, IsNil)

	do, err := vol.ToDriverOptions(1)
	c.Assert(err, IsNil)

	expected := storage.DriverOptions{
		Volume: storage.Volume{
			Name:   "basic/test",
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

func (s *testSuite) TestMountSource(c *C) {
	c.Assert(s.client.Set(testPolicies["nfs"]), IsNil)
	vol, err := db.CreateVolume(&db.VolumeRequest{Policy: testPolicies["nfs"], Name: "test", Options: map[string]string{"mount": "localhost:/mnt"}})
	c.Assert(err, IsNil)
	c.Assert(vol.MountSource, Equals, "localhost:/mnt")
	_, err = db.CreateVolume(&db.VolumeRequest{Policy: testPolicies["nfs"], Name: "test2"})
	c.Assert(err, NotNil)
	copy := testPolicies["basic"].Copy()
	copy.(*db.Policy).Name = "nfs"
	c.Assert(s.client.Set(testPolicies["basic"]), IsNil)
	vol, err = db.CreateVolume(&db.VolumeRequest{Policy: copy.(*db.Policy), Name: "test2"})
	c.Assert(err, IsNil)
	c.Assert(vol.MountSource, Equals, "")
}

func (s *testSuite) TestWatchVolumes(c *C) {
	entChan, errChan := s.client.WatchPrefix(&db.Volume{})
	select {
	case err := <-errChan:
		c.Assert(err, IsNil)
	default:
	}

	c.Assert(s.client.Set(testPolicies["basic"]), IsNil)
	vol, err := db.CreateVolume(&db.VolumeRequest{Policy: testPolicies["basic"], Name: "test"})
	c.Assert(err, IsNil)

	for i := 0; i < 5; i++ {
		vol.VolumeName = fmt.Sprintf("test%d", i)
		c.Assert(s.client.Set(vol), IsNil)

		select {
		case err := <-errChan:
			c.Assert(err, IsNil)
		case ent := <-entChan:
			logrus.Infof("Received object for %v during prefix watch", ent)
			vol2 := ent.(*db.Volume)
			c.Assert(vol, DeepEquals, vol2)
		}

		c.Assert(s.client.Delete(vol), IsNil)
		time.Sleep(200 * time.Millisecond) // wait for watch
		select {
		case <-errChan:
			panic("error received after delete in watch")
		case <-entChan:
			panic("object received after delete in watch")
		default:
			logrus.Info("Watch delete was processed successfully")
		}
	}

	c.Assert(s.client.WatchPrefixStop(&db.Volume{}), IsNil)

	vol, err = db.CreateVolume(&db.VolumeRequest{Policy: testPolicies["basic"], Name: "test2"})
	c.Assert(err, IsNil)
	c.Assert(s.client.Set(vol), IsNil)

	select {
	case <-entChan:
		c.Assert(false, Equals, true, Commentf("received on a should-be-closed channel"))
	default:
	}
}
