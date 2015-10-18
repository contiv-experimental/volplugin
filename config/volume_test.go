package config

import . "gopkg.in/check.v1"

func (s *configSuite) TestVolumeConfigValidate(c *C) {
	vc := &VolumeConfig{
		Options:    nil,
		VolumeName: "foo",
	}
	c.Assert(vc.Validate(), NotNil)

	vc = &VolumeConfig{
		Options:    &VolumeOptions{Size: 10, UseSnapshots: false},
		VolumeName: "",
	}

	c.Assert(vc.Validate(), NotNil)

	vc = &VolumeConfig{
		Options:    &VolumeOptions{Size: 10, UseSnapshots: false},
		VolumeName: "foo",
	}

	c.Assert(vc.Validate(), IsNil)
}

func (s *configSuite) TestVolumeOptionsValidate(c *C) {
	opts := &VolumeOptions{}
	c.Assert(opts.Validate(), NotNil)

	opts = &VolumeOptions{Size: 0}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: 10}
	c.Assert(opts.Validate(), IsNil)

	opts = &VolumeOptions{Size: 10, UseSnapshots: true}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: 10, UseSnapshots: true, Snapshot: SnapshotConfig{}}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: 10, UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "10m", Keep: 0}}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: 10, UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "", Keep: 10}}
	c.Assert(opts.Validate(), NotNil)
	opts = &VolumeOptions{Size: 10, UseSnapshots: true, Snapshot: SnapshotConfig{Frequency: "10m", Keep: 10}}
	c.Assert(opts.Validate(), IsNil)
}
