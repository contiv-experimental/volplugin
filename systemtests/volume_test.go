package systemtests

import (
	"strings"

	"github.com/contiv/vagrantssh"

	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestVolumeCreate(c *C) {
	c.Assert(s.createVolumeHost("policy1", "mon0", nil), IsNil)
}

func (s *systemtestSuite) TestVolumeCreateMultiHost(c *C) {
	for _, host := range []string{"mon0", "mon1", "mon2"} {
		c.Assert(s.createVolumeHost("policy1", host, nil), IsNil)
	}
}

func (s *systemtestSuite) TestVolumeCreateMultiHostCrossHostMount(c *C) {
	if nullDriver() {
		c.Skip("This driver does not support multi-host operation")
		return
	}

	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)

	_, err := s.dockerRun("mon0", false, false, "policy1/test", `sh -c "echo bar > /mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(s.createVolume("mon1", "policy1", "test", nil), IsNil)

	out, err := s.dockerRun("mon1", false, false, "policy1/test", `sh -c "cat /mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "bar")

	c.Assert(s.createVolume("mon1", "policy1", "test", nil), IsNil)

	_, err = s.dockerRun("mon1", false, false, "policy1/test", `sh -c "echo quux > /mnt/foo"`)
	c.Assert(err, IsNil)

	c.Assert(s.createVolume("mon2", "policy1", "test", nil), IsNil)

	out, err = s.dockerRun("mon2", false, false, "policy1/test", `sh -c "cat /mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "quux")
}

func (s *systemtestSuite) TestVolumeMultiPolicyCreate(c *C) {
	_, err := s.uploadIntent("policy2", "policy2")
	c.Assert(err, IsNil)

	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)
	c.Assert(s.createVolume("mon0", "policy2", "test", nil), IsNil)

	_, err = s.dockerRun("mon0", false, false, "policy1/test", `sh -c "echo foo > /mnt/bar"`)
	c.Assert(err, IsNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.dockerRun("mon0", false, false, "policy2/test", `sh -c "cat /mnt/bar"`)
	c.Assert(err, NotNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.dockerRun("mon0", false, false, "policy2/test", `sh -c "echo bar > /mnt/foo"`)
	c.Assert(err, IsNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.dockerRun("mon0", false, false, "policy1/test", `sh -c "cat /mnt/foo"`)
	c.Assert(err, NotNil)
}

func (s *systemtestSuite) TestVolumeMultiCreateThroughDocker(c *C) {
	if !cephDriver() {
		c.Skip("This is only supported by the ceph driver")
		return
	}

	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "policy1.test")

	c.Assert(s.vagrant.IterateNodes(func(node vagrantssh.TestbedNode) error {
		return node.RunCommand("docker volume create -d volplugin --name policy1/test")
	}), IsNil)

	c.Assert(s.vagrant.IterateNodes(func(node vagrantssh.TestbedNode) error {
		return node.RunCommand("docker volume rm policy1/test")
	}), IsNil)
}
