package systemtests

import (
	"encoding/json"
	"strings"
	"time"

	. "gopkg.in/check.v1"

	"github.com/contiv/volplugin/config"
)

func (s *systemtestSuite) TestEtcdUpdate(c *C) {
	// this not-very-obvious test ensures that the tenant can be uploaded after
	// the volplugin/volmaster pair are started.
	defer s.purgeVolume("mon0", "tenant1", "foo", true)
	c.Assert(s.createVolume("mon0", "tenant1", "foo", nil), IsNil)
}

func (s *systemtestSuite) TestSnapshotSchedule(c *C) {
	_, err := s.uploadIntent("tenant1", "fastsnap")
	c.Assert(err, IsNil)
	c.Assert(s.createVolume("mon0", "tenant1", "foo", nil), IsNil)
	defer s.purgeVolume("mon0", "tenant1", "foo", true)

	time.Sleep(2 * time.Second)

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd snap ls foo")
	c.Assert(err, IsNil)
	c.Assert(len(strings.Split(out, "\n")), Not(Equals), 0)
}

func (s *systemtestSuite) TestHostLabel(c *C) {
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)

	_, err := s.vagrant.GetNode("mon0").RunCommandBackground("sudo -E `which volplugin` --host-label quux --debug tenant1")
	c.Assert(err, IsNil)

	time.Sleep(10 * time.Millisecond)
	c.Assert(s.createVolume("mon0", "tenant1", "foo", nil), IsNil)

	out, err := s.docker("run -d -v tenant1/foo:/mnt ubuntu sleep infinity")
	c.Assert(err, IsNil)

	defer s.purgeVolume("mon0", "tenant1", "foo", true)
	defer s.docker("rm -f " + out)

	mt := &config.MountConfig{}

	// we know the pool is rbd here, so cheat a little.
	out, err = s.volcli("mount get rbd foo")
	c.Assert(err, IsNil)
	c.Assert(json.Unmarshal([]byte(out), mt), IsNil)
	c.Assert(mt.Host, Equals, "quux")
}

func (s *systemtestSuite) TestMountLock(c *C) {
	c.Assert(s.createVolume("mon0", "tenant1", "test", nil), IsNil)

	defer s.purgeVolume("mon0", "tenant1", "test", true)

	for _, name := range []string{"mon1", "mon2"} {
		c.Assert(s.createVolume(name, "tenant1", "test", nil), IsNil)
		defer s.purgeVolume(name, "tenant1", "test", false)
	}

	defer s.clearContainers()

	dockerCmd := "docker run -d -v tenant1/test:/mnt ubuntu sleep infinity"
	c.Assert(s.vagrant.GetNode("mon0").RunCommand(dockerCmd), IsNil)

	for _, nodeName := range []string{"mon1", "mon2"} {
		_, err := s.vagrant.GetNode(nodeName).RunCommandWithOutput(dockerCmd)
		c.Assert(err, NotNil)
	}

	c.Assert(s.clearContainers(), IsNil)
	c.Assert(s.vagrant.GetNode("mon1").RunCommand(dockerCmd), IsNil)

	defer s.purgeVolume("mon1", "tenant1", "test", false)

	for _, nodeName := range []string{"mon0", "mon2"} {
		_, err := s.vagrant.GetNode(nodeName).RunCommandWithOutput(dockerCmd)
		c.Assert(err, NotNil)
	}
}

func (s *systemtestSuite) TestMultiPool(c *C) {
	_, err := s.mon0cmd("sudo ceph osd pool create test 1 1")
	c.Assert(err, IsNil)
	defer s.mon0cmd("sudo ceph osd pool delete test test --yes-i-really-really-mean-it")

	c.Assert(s.createVolume("mon0", "tenant1", "test", map[string]string{"pool": "test"}), IsNil)
	defer s.purgeVolume("mon0", "tenant1", "test", true)

	out, err := s.volcli("volume get tenant1 test")
	c.Assert(err, IsNil)

	vc := &config.VolumeConfig{}
	c.Assert(json.Unmarshal([]byte(out), vc), IsNil)
	c.Assert(vc.Options.Size, Equals, uint64(10))
}

func (s *systemtestSuite) TestDriverOptions(c *C) {
	opts := map[string]string{
		"size":                "200",
		"snapshots":           "true",
		"snapshots.frequency": "100m",
		"snapshots.keep":      "20",
	}

	c.Assert(s.createVolume("mon0", "tenant1", "test", opts), IsNil)

	defer s.purgeVolume("mon0", "tenant1", "test", true)

	out, err := s.volcli("volume get tenant1 test")
	c.Assert(err, IsNil)

	vc := &config.VolumeConfig{}
	c.Assert(json.Unmarshal([]byte(out), vc), IsNil)
	c.Assert(vc.Options.Size, Equals, uint64(200))
	c.Assert(vc.Options.Snapshot.Frequency, Equals, "100m")
	c.Assert(vc.Options.Snapshot.Keep, Equals, uint(20))
}

func (s *systemtestSuite) TestMultipleFileSystems(c *C) {
	_, err := s.uploadIntent("tenant1", "fs")
	c.Assert(err, IsNil)

	opts := map[string]string{
		"size": "1000",
	}

	c.Assert(s.createVolume("mon0", "tenant1", "test", opts), IsNil)
	defer s.purgeVolume("mon0", "tenant1", "test", true)

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker run -d -v tenant1/test:/mnt ubuntu sleep infinity"), IsNil)

	defer s.clearContainers()

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("mount -l -t btrfs")
	c.Assert(err, IsNil)

	lines := strings.Split(out, "\n")
	pass := false
	for _, line := range lines {
		// cheat.
		if strings.Contains(line, "/dev/rbd") {
			pass = true
			break
		}
	}

	c.Assert(pass, Equals, true)
	c.Assert(s.createVolume("mon0", "tenant1", "testext4", map[string]string{"filesystem": "ext4"}), IsNil)

	defer s.purgeVolume("mon0", "tenant1", "testext4", true)

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker run -d -v tenant1/testext4:/mnt ubuntu sleep infinity"), IsNil)

	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("mount -l -t ext4")
	c.Assert(err, IsNil)

	lines = strings.Split(out, "\n")
	pass = false
	for _, line := range lines {
		// cheat.
		if strings.Contains(line, "/dev/rbd") {
			pass = true
			break
		}
	}

	c.Assert(pass, Equals, true)
}

func (s *systemtestSuite) TestMultiTenantVolumeCreate(c *C) {
	_, err := s.uploadIntent("tenant2", "intent2")
	c.Assert(err, IsNil)

	c.Assert(s.createVolume("mon0", "tenant1", "test", nil), IsNil)
	c.Assert(s.createVolume("mon0", "tenant2", "test", nil), IsNil)

	defer s.purgeVolume("mon0", "tenant1", "test", true)
	defer s.purgeVolume("mon0", "tenant2", "test", true)

	_, err = s.docker("run -v tenant1/test:/mnt ubuntu sh -c \"echo foo > /mnt/bar\"")
	c.Assert(err, IsNil)

	c.Assert(s.clearContainers(), IsNil)

	out, err := s.docker("run -v tenant2/test:/mnt ubuntu sh -c \"cat /mnt/bar\"")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "foo")
}

func (s *systemtestSuite) TestEphemeralVolumes(c *C) {
	_, err := s.uploadIntent("tenant1", "intent1")
	c.Assert(err, IsNil)

	c.Assert(s.createVolume("mon0", "tenant1", "test", map[string]string{"ephemeral": "true"}), IsNil)
	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "test")

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker volume rm tenant1/test"), IsNil)
	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Not(Equals), "test")

	c.Assert(s.createVolume("mon0", "tenant1", "test", map[string]string{"ephemeral": "false"}), IsNil)
	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "test")

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker volume rm tenant1/test"), IsNil)
	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "test")
}
