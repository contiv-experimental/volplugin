package systemtests

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	. "gopkg.in/check.v1"

	"github.com/contiv/volplugin/config"
)

func (s *systemtestSuite) TestIntegratedEtcdUpdate(c *C) {
	// this not-very-obvious test ensures that the policy can be uploaded after
	// the volplugin/volmaster pair are started.
	c.Assert(s.createVolume("mon0", "policy1", "foo", nil), IsNil)
}

func (s *systemtestSuite) TestIntegratedUseMountLock(c *C) {
	for _, name := range []string{"mon0", "mon1", "mon2"} {
		c.Assert(s.createVolume(name, "policy1", "test", nil), IsNil)
	}

	dockerCmd := "docker run -d -v policy1/test:/mnt alpine sleep 10m"
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
	c.Assert(err, IsNil)

	_, err = s.uploadIntent("testpool", "testpool")
	c.Assert(err, IsNil)

	c.Assert(s.createVolume("mon0", "testpool", "test", nil), IsNil)

	out, err := s.volcli("volume get testpool/test")
	c.Assert(err, IsNil)

	vc := &config.Volume{}
	c.Assert(json.Unmarshal([]byte(out), vc), IsNil)
	actualSize, err := vc.CreateOptions.ActualSize()
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

	c.Assert(s.createVolume("mon0", "policy1", "test", opts), IsNil)

	defer s.purgeVolume("mon0", "policy1", "test", true)

	out, err := s.volcli("volume get policy1/test")
	c.Assert(err, IsNil)

	vc := &config.Volume{}
	c.Assert(json.Unmarshal([]byte(out), vc), IsNil)

	actualSize, err := vc.CreateOptions.ActualSize()
	c.Assert(err, IsNil)
	c.Assert(actualSize, Equals, uint64(200))
	c.Assert(vc.RuntimeOptions.Snapshot.Frequency, Equals, "100m")
	c.Assert(vc.RuntimeOptions.Snapshot.Keep, Equals, uint(20))
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

	c.Assert(s.createVolume("mon0", "policy1", "test", opts), IsNil)
	_, err := s.docker("run -itd -v policy1/test:/mnt alpine sleep 10m")
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

	s.volcli("volume runtime upload policy1/test < /testdata/iops1.json")
	// copied from iops1.json
	opts = map[string]string{
		"rate-limit.write.bps":  "1000000",
		"rate-limit.write.iops": "1000",
		"rate-limit.read.bps":   "10",
		"rate-limit.read.iops":  "1200",
	}

	time.Sleep(30 * time.Second) // TTL

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
	c.Assert(s.uploadGlobal("global-fasttimeout"), IsNil)

	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)
	_, err := s.docker("run -itd -v policy1/test:/mnt alpine sleep 10m")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume remove policy1/test")
	c.Assert(err, NotNil)

	s.clearContainers()

	_, err = s.volcli("volume remove policy1/test")
	c.Assert(err, IsNil)
}

func (s *systemtestSuite) TestIntegratedVolumeSnapshotCopy(c *C) {
	_, err := s.uploadIntent("policy1", "fastsnap")
	c.Assert(err, IsNil)
	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)

	time.Sleep(4 * time.Second)

	out, err := s.volcli("volume snapshot list policy1/test")
	c.Assert(err, IsNil)

	lines := strings.Split(out, "\n")
	c.Assert(len(lines), Not(Equals), 0)

	_, err = s.volcli(fmt.Sprintf("volume snapshot copy policy1/test %s test2", lines[0]))
	c.Assert(err, IsNil)

	defer func() {
		c.Assert(s.purgeVolume("mon0", "policy1", "test2", true), IsNil)
		_, err := s.mon0cmd(fmt.Sprintf("sudo rbd snap unprotect policy1.test --snap %s --pool rbd", lines[0]))
		c.Assert(err, IsNil)
		c.Assert(s.purgeVolume("mon0", "policy1", "test", true), IsNil)
	}()

	out, err = s.volcli("volume list-all")
	c.Assert(err, IsNil)

	lines2 := strings.Split(out, "\n")

	sort.Strings(lines2)

	// off-by-one because of the initial newline in the volcli output
	c.Assert([]string{"policy1/test", "policy1/test2"}, DeepEquals, lines2[1:])
}

func (s *systemtestSuite) TestIntegratedVolumeSnapshotCopyFailures(c *C) {
	_, err := s.uploadIntent("policy1", "fastsnap")
	c.Assert(err, IsNil)
	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)

	time.Sleep(4 * time.Second)

	out, err := s.volcli("volume snapshot list policy1/test")
	c.Assert(err, IsNil)

	lines := strings.Split(out, "\n")
	c.Assert(len(lines), Not(Equals), 0)

	_, err = s.volcli(fmt.Sprintf("volume snapshot copy policy1/test %s test", lines[0]))
	c.Assert(err, NotNil)
	_, err = s.volcli("volume snapshot copy policy1/test foo test2")
	c.Assert(err, NotNil)
	_, err = s.volcli(fmt.Sprintf("volume snapshot copy policy1/nonexistent %s test2", lines[0]))
	c.Assert(err, NotNil)
}

func (s *systemtestSuite) TestIntegratedMultipleFileSystems(c *C) {
	_, err := s.uploadIntent("policy2", "fs")
	c.Assert(err, IsNil)

	opts := map[string]string{
		"size": "1GB",
	}

	c.Assert(s.createVolume("mon0", "policy2", "test", opts), IsNil)
	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker run -d -v policy2/test:/mnt alpine sleep 10m"), IsNil)

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
	c.Assert(s.createVolume("mon0", "policy2", "testext4", map[string]string{"filesystem": "ext4"}), IsNil)

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker run -d -v policy2/testext4:/mnt alpine sleep 10m"), IsNil)

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
