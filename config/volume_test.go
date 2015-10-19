package config

import (
	"path"
	"sort"

	. "gopkg.in/check.v1"
)

func (s *configSuite) TestVolumeConfigValidate(c *C) {
	vc := &VolumeConfig{
		Options:    nil,
		VolumeName: "foo",
	}
	c.Assert(vc.Validate(), NotNil)

	vc = &VolumeConfig{
		Options:    &VolumeOptions{Size: 10, UseSnapshots: false, Pool: "rbd"},
		VolumeName: "",
	}

	c.Assert(vc.Validate(), NotNil)

	vc = &VolumeConfig{
		Options:    &VolumeOptions{Size: 10, UseSnapshots: false, Pool: "rbd"},
		VolumeName: "foo",
	}

	c.Assert(vc.Validate(), IsNil)
}

func (s *configSuite) TestVolumeOptionsValidate(c *C) {
	opts := &VolumeOptions{}
	c.Assert(opts.Validate(), NotNil)

	opts = &VolumeOptions{Size: 0}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: 10, Pool: "rbd"}
	c.Assert(opts.Validate(), IsNil)

	opts = &VolumeOptions{Size: 10, UseSnapshots: true, Pool: "rbd"}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: 10, UseSnapshots: true, Snapshot: SnapshotConfig{}, Pool: "rbd"}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: 10, UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "10m", Keep: 0}, Pool: "rbd"}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: 10, UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "", Keep: 10}, Pool: "rbd"}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: 10, UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "10m", Keep: 10}, Pool: "rbd"}
	c.Assert(opts.Validate(), IsNil)
}

func (s *configSuite) TestVolumeCRUD(c *C) {
	tenantNames := []string{"foo", "bar"}
	volumeNames := []string{"baz", "quux"}
	sort.Strings(volumeNames) // lazy

	for _, tenant := range tenantNames {
		c.Assert(s.tlc.PublishTenant(tenant, testTenantConfigs["basic"]), IsNil)

		for _, volume := range volumeNames {
			vcfg, err := s.tlc.CreateVolume(RequestCreate{Tenant: tenant, Volume: volume})
			c.Assert(err, IsNil)

			defer func(tenant, volume string) { c.Assert(s.tlc.RemoveVolume(tenant, volume), IsNil) }(tenant, volume)

			c.Assert(vcfg.VolumeName, Equals, volume)
			opts := testTenantConfigs["basic"].DefaultVolumeOptions
			opts.Pool = "rbd"
			c.Assert(vcfg.Options, DeepEquals, &opts)

			vcfg2, err := s.tlc.GetVolume(tenant, volume)
			c.Assert(err, IsNil)

			c.Assert(vcfg, DeepEquals, vcfg2)
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
