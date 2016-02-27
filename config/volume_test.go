package config

import (
	"path"
	"sort"

	. "gopkg.in/check.v1"
)

func (s *configSuite) TestActualSize(c *C) {
	vo := &VolumeOptions{Size: "10MB", UseSnapshots: false, Pool: "rbd"}
	actualSize, err := vo.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(int(actualSize), Equals, 10)

	vo = &VolumeOptions{Size: "1GB", UseSnapshots: false, Pool: "rbd"}
	actualSize, err = vo.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(int(actualSize), Equals, 1024)

	vo = &VolumeOptions{Size: "0", UseSnapshots: false, Pool: "rbd"}
	actualSize, err = vo.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(int(actualSize), Equals, 0)

	vo = &VolumeOptions{Size: "10M", UseSnapshots: false, Pool: "rbd"}
	_, err = vo.ActualSize()
	c.Assert(err, NotNil)

	vo = &VolumeOptions{Size: "garbage", UseSnapshots: false, Pool: "rbd"}
	_, err = vo.ActualSize()
	c.Assert(err, NotNil)
}

func (s *configSuite) TestVolumeConfigValidate(c *C) {
	vc := &VolumeConfig{
		Options:    nil,
		VolumeName: "foo",
		TenantName: "tenant1",
	}
	c.Assert(vc.Validate(), NotNil)

	vc = &VolumeConfig{
		Options:    &VolumeOptions{Size: "10MB", UseSnapshots: false, Pool: "rbd", actualSize: 10},
		VolumeName: "",
		TenantName: "tenant1",
	}

	c.Assert(vc.Validate(), NotNil)

	vc = &VolumeConfig{
		Options:    &VolumeOptions{Size: "10MB", UseSnapshots: false, Pool: "rbd", actualSize: 10},
		VolumeName: "foo",
		TenantName: "",
	}

	c.Assert(vc.Validate(), NotNil)

	vc = &VolumeConfig{
		Options:    &VolumeOptions{Size: "10MB", UseSnapshots: false, Pool: "rbd", actualSize: 10},
		VolumeName: "foo",
		TenantName: "tenant1",
	}

	c.Assert(vc.Validate(), IsNil)
}

func (s *configSuite) TestVolumeOptionsValidate(c *C) {
	opts := &VolumeOptions{}
	c.Assert(opts.Validate(), NotNil)

	opts = &VolumeOptions{Size: "0"}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: "10MB", Pool: "rbd", actualSize: 10}
	c.Assert(opts.Validate(), IsNil)

	opts = &VolumeOptions{Size: "10MB", UseSnapshots: true, Pool: "rbd", actualSize: 10}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: "10MB", UseSnapshots: true, Snapshot: SnapshotConfig{}, Pool: "rbd", actualSize: 10}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: "10MB", UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "10m", Keep: 0}, Pool: "rbd", actualSize: 10}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: "10MB", UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "", Keep: 10}, Pool: "rbd", actualSize: 10}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: "10MB", UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "10m", Keep: 10}, Pool: "rbd", actualSize: 10}
	c.Assert(opts.Validate(), IsNil)
}

func (s *configSuite) TestVolumeCRUD(c *C) {
	tenantNames := []string{"foo", "bar"}
	volumeNames := []string{"baz", "quux"}
	sort.Strings(volumeNames) // lazy

	_, err := s.tlc.CreateVolume(RequestCreate{})
	c.Assert(err, NotNil)

	_, err = s.tlc.CreateVolume(RequestCreate{Tenant: "Doesn'tExist"})
	c.Assert(err, NotNil)

	// populate the tenants so the next few tests don't give false positives
	for _, tenant := range tenantNames {
		c.Assert(s.tlc.PublishTenant(tenant, testTenantConfigs["basic"]), IsNil)
	}

	_, err = s.tlc.CreateVolume(RequestCreate{Tenant: "foo", Volume: "bar", Opts: map[string]string{"quux": "derp"}})
	c.Assert(err, NotNil)

	_, err = s.tlc.CreateVolume(RequestCreate{Tenant: "foo", Volume: "bar", Opts: map[string]string{"pool": ""}})
	c.Assert(err, NotNil)

	_, err = s.tlc.CreateVolume(RequestCreate{Tenant: "foo", Volume: ""})
	c.Assert(err, NotNil)

	_, err = s.tlc.GetVolume("foo", "bar")
	c.Assert(err, NotNil)

	_, err = s.tlc.ListVolumes("quux")
	c.Assert(err, NotNil)

	for _, tenant := range tenantNames {
		for _, volume := range volumeNames {
			vcfg, err := s.tlc.CreateVolume(RequestCreate{Tenant: tenant, Volume: volume, Opts: map[string]string{"filesystem": ""}})
			c.Assert(err, IsNil)
			c.Assert(s.tlc.PublishVolume(vcfg), IsNil)
			c.Assert(s.tlc.PublishVolume(vcfg), Equals, ErrExist)

			c.Assert(vcfg.Options.FileSystem, Equals, "ext4")

			defer func(tenant, volume string) { c.Assert(s.tlc.RemoveVolume(tenant, volume), IsNil) }(tenant, volume)

			c.Assert(vcfg.VolumeName, Equals, volume)
			opts := testTenantConfigs["basic"].DefaultVolumeOptions
			opts.Pool = "rbd"
			c.Assert(vcfg.Options, DeepEquals, &opts)

			vcfg2, err := s.tlc.GetVolume(tenant, volume)
			c.Assert(err, IsNil)

			c.Assert(vcfg, DeepEquals, vcfg2)

			vcfg.Options.Size = "0"
			vcfg.Options.actualSize = 0
			c.Assert(s.tlc.PublishVolume(vcfg), NotNil)
		}

		volumes, err := s.tlc.ListVolumes(tenant)
		c.Assert(err, IsNil)

		volumeKeys := []string{}

		for key := range volumes {
			volumeKeys = append(volumeKeys, key)
		}

		sort.Strings(volumeKeys)

		c.Assert(volumeNames, DeepEquals, volumeKeys)
	}

	allVols, err := s.tlc.ListAllVolumes()
	c.Assert(err, IsNil)

	sort.Strings(allVols)

	allNames := []string{}

	for _, tenant := range tenantNames {
		for _, volume := range volumeNames {
			allNames = append(allNames, path.Join(tenant, volume))
		}
	}

	sort.Strings(allNames)

	c.Assert(allNames, DeepEquals, allVols)
}
