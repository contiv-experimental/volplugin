package config

import (
	"os/exec"
	"time"

	. "gopkg.in/check.v1"
)

var testTenantConfigs = map[string]*TenantConfig{
	"basic": {
		DefaultVolumeOptions: VolumeOptions{
			Pool:         "rbd",
			Size:         "10MB",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"basic2": {
		DefaultVolumeOptions: VolumeOptions{
			Pool:         "rbd",
			Size:         "20MB",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"untouchedwithzerosize": {
		DefaultVolumeOptions: VolumeOptions{
			Pool:         "rbd",
			Size:         "0",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"nilfs": {
		DefaultVolumeOptions: VolumeOptions{
			Pool:         "rbd",
			Size:         "20MB",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"nopool": {
		DefaultVolumeOptions: VolumeOptions{
			Size:         "20MB",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsize": {
		DefaultVolumeOptions: VolumeOptions{
			Size:         "0",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsize2": {
		DefaultVolumeOptions: VolumeOptions{
			Size:         "10M",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsize3": {
		DefaultVolumeOptions: VolumeOptions{
			Size:         "not a number",
			UseSnapshots: false,
			FileSystem:   defaultFilesystem,
		},
		FileSystems: defaultFilesystems,
	},
	"badsnaps": {
		DefaultVolumeOptions: VolumeOptions{
			Size:         "10M",
			UseSnapshots: true,
			FileSystem:   defaultFilesystem,
			Snapshot: SnapshotConfig{
				Keep:      0,
				Frequency: "",
			},
		},
		FileSystems: defaultFilesystems,
	},
}

func (s *configSuite) TestBasicTenant(c *C) {
	c.Assert(s.tlc.PublishTenant("quux", testTenantConfigs["basic"]), IsNil)

	cfg, err := s.tlc.GetTenant("quux")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testTenantConfigs["basic"])

	c.Assert(s.tlc.PublishTenant("bar", testTenantConfigs["basic2"]), IsNil)

	cfg, err = s.tlc.GetTenant("bar")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testTenantConfigs["basic2"])

	tenants, err := s.tlc.ListTenants()
	c.Assert(err, IsNil)

	for _, tenant := range tenants {
		found := false
		for _, name := range []string{"bar", "quux"} {
			if tenant == name {
				found = true
			}
		}

		c.Assert(found, Equals, true)
	}

	c.Assert(s.tlc.DeleteTenant("bar"), IsNil)
	_, err = s.tlc.GetTenant("bar")
	c.Assert(err, NotNil)

	cfg, err = s.tlc.GetTenant("quux")
	c.Assert(err, IsNil)
	c.Assert(cfg, DeepEquals, testTenantConfigs["basic"])
}

func (s *configSuite) TestTenantValidate(c *C) {
	for _, key := range []string{"basic", "basic2", "nilfs"} {
		c.Assert(testTenantConfigs[key].Validate(), IsNil)
	}

	// FIXME: ensure the default filesystem option is set when validate is called.
	//        honestly, this both a pretty lousy way to do it and test it, we should do
	//        something better.
	c.Assert(testTenantConfigs["nilfs"].FileSystems, DeepEquals, map[string]string{defaultFilesystem: "mkfs.ext4 -m0 %"})

	c.Assert(testTenantConfigs["untouchedwithzerosize"].Validate(), NotNil)
	c.Assert(testTenantConfigs["nopool"].Validate(), NotNil)
	c.Assert(testTenantConfigs["badsize"].Validate(), NotNil)
	c.Assert(testTenantConfigs["badsize2"].Validate(), NotNil)
	_, err := testTenantConfigs["badsize3"].DefaultVolumeOptions.ActualSize()
	c.Assert(err, NotNil)
}

func (s *configSuite) TestTenantBadPublish(c *C) {
	for _, key := range []string{"badsize", "badsize2", "badsize3", "nopool", "badsnaps"} {
		c.Assert(s.tlc.PublishTenant("test", testTenantConfigs[key]), NotNil)
	}
}

func (s *configSuite) TestTenantPublishEtcdDown(c *C) {
	defer time.Sleep(1 * time.Second)
	defer func() {
		// I have ABSOLUTELY no idea why CombinedOutput() is required here, but it is.
		_, err := exec.Command("/bin/sh", "-c", "sudo systemctl restart etcd").CombinedOutput()
		c.Assert(err, IsNil)
	}()

	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl start etcd").Run(), IsNil)
	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl stop etcd").Run(), IsNil)
	time.Sleep(1 * time.Second)

	for _, key := range []string{"basic", "basic2"} {
		c.Assert(s.tlc.PublishTenant("test", testTenantConfigs[key]), NotNil)
	}
}

func (s *configSuite) TestTenantListEtcdDown(c *C) {
	defer time.Sleep(1 * time.Second)
	defer func() {
		// I have ABSOLUTELY no idea why CombinedOutput() is required here, but it is.
		_, err := exec.Command("/bin/sh", "-c", "sudo systemctl restart etcd").CombinedOutput()
		c.Assert(err, IsNil)
	}()

	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl start etcd").Run(), IsNil)
	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl stop etcd").Run(), IsNil)
	time.Sleep(1 * time.Second)

	_, err := s.tlc.ListTenants()

	c.Assert(err, NotNil)
}
