package systemtests

import (
	"strings"
	"time"

	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestVolsupervisorSnapshotSchedule(c *C) {
	if !cephDriver() {
		c.Skip("Only ceph supports snapshots")
		return
	}

	_, err := s.uploadIntent("policy1", "fastsnap")
	c.Assert(err, IsNil)

	volName := genRandomVolume()

	c.Assert(s.createVolume("mon0", fqVolume("policy1", volName), nil), IsNil)

	time.Sleep(4 * time.Second)

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd snap ls policy1." + volName)
	c.Assert(err, IsNil)
	c.Assert(len(strings.Split(out, "\n")) > 2, Equals, true)

	time.Sleep(15 * time.Second)

	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd snap ls policy1." + volName)
	c.Assert(err, IsNil)
	mylen := len(strings.Split(out, "\n"))
	c.Assert(mylen, Not(Equals), 0)
	c.Assert(mylen >= 5 && mylen <= 10, Equals, true)
}

func (s *systemtestSuite) TestVolsupervisorStopStartSnapshot(c *C) {
	if !cephDriver() {
		c.Skip("Only ceph supports snapshots")
		return
	}

	_, err := s.uploadIntent("policy1", "fastsnap")
	c.Assert(err, IsNil)

	volName := genRandomVolume()
	fqVolName := fqVolume("policy1", volName)

	c.Assert(s.createVolume("mon0", fqVolName, nil), IsNil)

	time.Sleep(4 * time.Second)

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd snap ls policy1." + volName)
	c.Assert(err, IsNil)
	c.Assert(len(strings.Split(out, "\n")) > 2, Equals, true)

	out, err = s.volcli("volume remove " + fqVolName)
	c.Assert(err, IsNil)

	_, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd snap ls policy1." + volName)
	c.Assert(err, NotNil)

	_, err = s.uploadIntent("policy1", "nosnap")
	c.Assert(err, IsNil)

	// XXX we don't use createVolume here because of a bug in docker that doesn't
	// allow it to create the same volume twice
	_, err = s.volcli("volume create " + fqVolName)
	c.Assert(err, IsNil)

	time.Sleep(4 * time.Second)

	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd snap ls policy1." + volName)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)
}

func (s *systemtestSuite) TestVolsupervisorRestart(c *C) {
	if !cephDriver() {
		c.Skip("Only ceph supports snapshots")
		return
	}

	_, err := s.uploadIntent("policy1", "fastsnap")
	c.Assert(err, IsNil)

	volName := genRandomVolume()
	fqVolName := fqVolume("policy1", volName)

	c.Assert(s.createVolume("mon0", fqVolName, nil), IsNil)

	time.Sleep(4 * time.Second)

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd snap ls policy1." + volName)
	c.Assert(err, IsNil)

	count := len(strings.Split(out, "\n"))
	c.Assert(count > 2, Equals, true)

	c.Assert(stopVolsupervisor(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(startVolsupervisor(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(waitForVolsupervisor(s.vagrant.GetNode("mon0")), IsNil)

	time.Sleep(10 * time.Second)

	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd snap ls policy1." + volName)
	c.Assert(err, IsNil)
	count2 := len(strings.Split(out, "\n"))
	c.Assert(count2 > count, Equals, true)
}

func (s *systemtestSuite) TestVolsupervisorSignal(c *C) {
	if !cephDriver() {
		c.Skip("Only ceph supports snapshots")
		return
	}

	_, err := s.uploadIntent("policy1", "nosnap")
	c.Assert(err, IsNil)

	volName := genRandomVolume()
	fqVolName := fqVolume("policy1", volName)

	c.Assert(s.createVolume("mon0", fqVolName, nil), IsNil)
	_, err = s.volcli("volume snapshot take " + fqVolName)
	c.Assert(err, IsNil)

	time.Sleep(5 * time.Second)

	out, err := s.volcli("volume snapshot list " + fqVolName)
	c.Assert(err, IsNil)
	c.Assert(len(strings.TrimSpace(out)), Not(Equals), 0, Commentf(out))
}
