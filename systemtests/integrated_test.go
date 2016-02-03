package systemtests

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	. "gopkg.in/check.v1"

	"github.com/contiv/volplugin/config"
)

func (s *systemtestSuite) TestIntegratedEtcdUpdate(c *C) {
	// this not-very-obvious test ensures that the tenant can be uploaded after
	// the volplugin/volmaster pair are started.
	c.Assert(s.createVolume("mon0", "tenant1", "foo", nil), IsNil)
}

func (s *systemtestSuite) TestIntegratedSnapshotSchedule(c *C) {
	_, err := s.uploadIntent("tenant1", "fastsnap")
	c.Assert(err, IsNil)
	c.Assert(s.createVolume("mon0", "tenant1", "foo", nil), IsNil)

	time.Sleep(2 * time.Second)

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd snap ls tenant1.foo")
	c.Assert(err, IsNil)
	c.Assert(len(strings.Split(out, "\n")) > 2, Equals, true)

	time.Sleep(15 * time.Second)

	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd snap ls tenant1.foo")
	c.Assert(err, IsNil)
	mylen := len(strings.Split(out, "\n"))
	c.Assert(mylen, Not(Equals), 0)
	c.Assert(mylen >= 5 && mylen <= 10, Equals, true)
}

func (s *systemtestSuite) TestIntegratedUseMountLock(c *C) {
	for _, name := range []string{"mon0", "mon1", "mon2"} {
		c.Assert(s.createVolume(name, "tenant1", "test", nil), IsNil)
	}

	dockerCmd := "docker run -d -v tenant1/test:/mnt alpine sleep 10m"
	c.Assert(s.vagrant.GetNode("mon0").RunCommand(dockerCmd), IsNil)

	for _, nodeName := range []string{"mon1", "mon2"} {
		_, err := s.vagrant.GetNode(nodeName).RunCommandWithOutput(dockerCmd)
		c.Assert(err, NotNil)
	}

	c.Assert(s.clearContainers(), IsNil)
	c.Assert(s.vagrant.GetNode("mon1").RunCommand(dockerCmd), IsNil)

	for _, nodeName := range []string{"mon0", "mon2"} {
		_, err := s.vagrant.GetNode(nodeName).RunCommandWithOutput(dockerCmd)
		c.Assert(err, NotNil)
	}
}

func (s *systemtestSuite) TestIntegratedMultiPool(c *C) {
	defer s.mon0cmd("sudo ceph osd pool delete test test --yes-i-really-really-mean-it")
	_, err := s.mon0cmd("sudo ceph osd pool create test 1 1")

	c.Assert(s.createVolume("mon0", "tenant1", "test", map[string]string{"pool": "test"}), IsNil)

	out, err := s.volcli("volume get tenant1/test")
	c.Assert(err, IsNil)

	vc := &config.VolumeConfig{}
	c.Assert(json.Unmarshal([]byte(out), vc), IsNil)
	actualSize, err := vc.Options.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(actualSize, Equals, uint64(10))
}

func (s *systemtestSuite) TestIntegratedDriverOptions(c *C) {
	opts := map[string]string{
		"size":                "200MB",
		"snapshots":           "true",
		"snapshots.frequency": "100m",
		"snapshots.keep":      "20",
	}

	c.Assert(s.createVolume("mon0", "tenant1", "test", opts), IsNil)

	defer s.purgeVolume("mon0", "tenant1", "test", true)

	out, err := s.volcli("volume get tenant1/test")
	c.Assert(err, IsNil)

	vc := &config.VolumeConfig{}
	c.Assert(json.Unmarshal([]byte(out), vc), IsNil)
	actualSize, err := vc.Options.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(actualSize, Equals, uint64(200))
	c.Assert(vc.Options.Snapshot.Frequency, Equals, "100m")
	c.Assert(vc.Options.Snapshot.Keep, Equals, uint(20))
}

func (s *systemtestSuite) TestIntegratedMultipleFileSystems(c *C) {
	_, err := s.uploadIntent("tenant2", "fs")
	c.Assert(err, IsNil)

	opts := map[string]string{
		"size": "1GB",
	}

	c.Assert(s.createVolume("mon0", "tenant2", "test", opts), IsNil)
	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker run -d -v tenant2/test:/mnt alpine sleep 10m"), IsNil)

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
	c.Assert(s.createVolume("mon0", "tenant2", "testext4", map[string]string{"filesystem": "ext4"}), IsNil)

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker run -d -v tenant2/testext4:/mnt alpine sleep 10m"), IsNil)

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

func (s *systemtestSuite) TestIntegratedRateLimiting(c *C) {
	// FIXME find a better place for these
	var (
		writeIOPSFile = "/sys/fs/cgroup/blkio/blkio.throttle.write_iops_device"
		readIOPSFile  = "/sys/fs/cgroup/blkio/blkio.throttle.read_iops_device"
		writeBPSFile  = "/sys/fs/cgroup/blkio/blkio.throttle.write_bps_device"
		readBPSFile   = "/sys/fs/cgroup/blkio/blkio.throttle.read_bps_device"
	)

	opts := map[string]string{
		"rate-limit.write.bps":  "100000",
		"rate-limit.write.iops": "110000",
		"rate-limit.read.bps":   "120000",
		"rate-limit.read.iops":  "130000",
	}

	optMap := map[string]string{
		"rate-limit.write.bps":  writeBPSFile,
		"rate-limit.write.iops": writeIOPSFile,
		"rate-limit.read.bps":   readBPSFile,
		"rate-limit.read.iops":  readIOPSFile,
	}

	c.Assert(s.createVolume("mon0", "tenant1", "test", opts), IsNil)
	_, err := s.docker("run -itd -v tenant1/test:/mnt alpine sleep 10m")
	c.Assert(err, IsNil)

	for key, fn := range optMap {
		out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput(fmt.Sprintf("sudo cat '%s'", fn))
		c.Assert(err, IsNil)
		var found bool

		for _, line := range strings.Split(out, "\n") {
			parts := strings.Split(line, " ")

			if len(parts) < 2 {
				continue
			}

			if parts[1] == opts[key] {
				found = true
			}
		}

		c.Assert(found, Equals, true)
	}
}

func (s *systemtestSuite) TestIntegratedRemoveWhileMount(c *C) {
	c.Assert(s.createVolume("mon0", "tenant1", "test", nil), IsNil)
	_, err := s.docker("run -itd -v tenant1/test:/mnt alpine sleep 10m")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume remove tenant1/test")
	c.Assert(err, NotNil)

	s.clearContainers()

	_, err = s.volcli("volume remove tenant1/test")
	c.Assert(err, IsNil)
}
