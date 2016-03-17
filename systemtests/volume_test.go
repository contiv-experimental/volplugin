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
	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)

	_, err := s.vagrant.GetNode("mon0").RunCommandWithOutput(`docker run --rm -i -v policy1/test:/mnt alpine sh -c "echo bar >/mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(s.createVolume("mon1", "policy1", "test", nil), IsNil)

	out, err := s.vagrant.GetNode("mon1").RunCommandWithOutput(`docker run --rm -i -v policy1/test:/mnt alpine sh -c "cat /mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "bar")

	c.Assert(s.createVolume("mon1", "policy1", "test", nil), IsNil)

	_, err = s.vagrant.GetNode("mon1").RunCommandWithOutput(`docker run --rm -i -v policy1/test:/mnt alpine sh -c "echo quux >/mnt/foo"`)
	c.Assert(err, IsNil)

	c.Assert(s.createVolume("mon2", "policy1", "test", nil), IsNil)

	out, err = s.vagrant.GetNode("mon2").RunCommandWithOutput(`docker run --rm -i -v policy1/test:/mnt alpine sh -c "cat /mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "quux")
}

func (s *systemtestSuite) TestVolumeMultiPolicyCreate(c *C) {
	_, err := s.uploadIntent("policy2", "policy2")
	c.Assert(err, IsNil)

	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)
	c.Assert(s.createVolume("mon0", "policy2", "test", nil), IsNil)

	_, err = s.docker("run -v policy1/test:/mnt alpine sh -c \"echo foo > /mnt/bar\"")
	c.Assert(err, IsNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.docker("run -v policy2/test:/mnt alpine sh -c \"cat /mnt/bar\"")
	c.Assert(err, NotNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.docker("run -v policy2/test:/mnt alpine sh -c \"echo bar > /mnt/foo\"")
	c.Assert(err, IsNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.docker("run -v policy1/test:/mnt alpine sh -c \"cat /mnt/foo\"")
	c.Assert(err, NotNil)
}

func (s *systemtestSuite) TestVolumeEphemeral(c *C) {
	c.Assert(s.createVolume("mon0", "policy1", "test", map[string]string{"ephemeral": "true"}), IsNil)
	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "policy1.test")

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker volume rm policy1/test"), IsNil)
	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Not(Equals), "policy1.test")

	c.Assert(s.createVolume("mon0", "policy1", "test", map[string]string{"ephemeral": "false"}), IsNil)
	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "policy1.test")

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker volume rm policy1/test"), IsNil)
	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "policy1.test")
}

func (s *systemtestSuite) TestVolumeParallelCreate(c *C) {
	c.Assert(s.vagrant.IterateNodes(func(node vagrantssh.TestbedNode) error {
		return node.RunCommand("docker volume create -d volplugin --name policy1/test")
	}), IsNil)

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "policy1.test")

	c.Assert(s.vagrant.IterateNodes(func(node vagrantssh.TestbedNode) error {
		return node.RunCommand("docker volume rm policy1/test")
	}), IsNil)
}
