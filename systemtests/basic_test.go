package systemtests

import (
	"strings"

	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestStarted(c *C) {
	c.Assert(s.vagrant.GetNode("mon0").RunCommand("pgrep -c volmaster"), IsNil)
	c.Assert(s.vagrant.SSHAllNodes("pgrep -c volplugin"), IsNil)
}

func (s *systemtestSuite) TestSSH(c *C) {
	c.Assert(s.vagrant.SSHAllNodes("/bin/echo"), IsNil)
}

func (s *systemtestSuite) TestVolumeCreate(c *C) {
	defer s.purgeVolumeHost("tenant1", "mon0", true)
	c.Assert(s.createVolumeHost("tenant1", "mon0", nil), IsNil)
}

func (s *systemtestSuite) TestVolumeCreateMultiHost(c *C) {
	hosts := []string{"mon0", "mon1", "mon2"}
	defer func() {
		for _, host := range hosts {
			s.purgeVolumeHost("tenant1", host, true)
		}
	}()

	for _, host := range []string{"mon0", "mon1", "mon2"} {
		c.Assert(s.createVolumeHost("tenant1", host, nil), IsNil)
	}
}

func (s *systemtestSuite) TestVolumeCreateMultiHostCrossHostMount(c *C) {
	c.Assert(s.createVolume("mon0", "tenant1", "test", nil), IsNil)

	_, err := s.vagrant.GetNode("mon0").RunCommandWithOutput(`docker run --rm -i -v tenant1/test:/mnt ubuntu sh -c "echo bar >/mnt/foo"`)
	c.Assert(err, IsNil)
	defer s.purgeVolume("mon0", "tenant1", "test", true) // cleanup
	c.Assert(s.createVolume("mon1", "tenant1", "test", nil), IsNil)

	out, err := s.vagrant.GetNode("mon1").RunCommandWithOutput(`docker run --rm -i -v tenant1/test:/mnt ubuntu sh -c "cat /mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "bar")

	c.Assert(s.createVolume("mon1", "tenant1", "test", nil), IsNil)

	_, err = s.vagrant.GetNode("mon1").RunCommandWithOutput(`docker run --rm -i -v tenant1/test:/mnt ubuntu sh -c "echo quux >/mnt/foo"`)
	c.Assert(err, IsNil)

	c.Assert(s.createVolume("mon2", "tenant1", "test", nil), IsNil)
	defer s.purgeVolume("mon2", "tenant1", "test", true)

	out, err = s.vagrant.GetNode("mon2").RunCommandWithOutput(`docker run --rm -i -v tenant1/test:/mnt ubuntu sh -c "cat /mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "quux")
}
