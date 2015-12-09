package systemtests

import (
	"strings"

	"github.com/contiv/systemtests-utils"

	. "gopkg.in/check.v1"
)

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

func (s *systemtestSuite) TestVolumeMultiTenantCreate(c *C) {
	_, err := s.uploadIntent("tenant2", "intent2")
	c.Assert(err, IsNil)

	c.Assert(s.createVolume("mon0", "tenant1", "test", nil), IsNil)
	c.Assert(s.createVolume("mon0", "tenant2", "test", nil), IsNil)

	defer s.purgeVolume("mon0", "tenant1", "test", true)
	defer s.purgeVolume("mon0", "tenant2", "test", true)

	_, err = s.docker("run -v tenant1/test:/mnt ubuntu sh -c \"echo foo > /mnt/bar\"")
	c.Assert(err, IsNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.docker("run -v tenant2/test:/mnt ubuntu sh -c \"cat /mnt/bar\"")
	c.Assert(err, NotNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.docker("run -v tenant2/test:/mnt ubuntu sh -c \"echo bar > /mnt/foo\"")
	c.Assert(err, IsNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.docker("run -v tenant1/test:/mnt ubuntu sh -c \"cat /mnt/foo\"")
	c.Assert(err, NotNil)
}

func (s *systemtestSuite) TestVolumeEphemeral(c *C) {
	defer s.purgeVolume("mon0", "tenant1", "test", true)

	c.Assert(s.createVolume("mon0", "tenant1", "test", map[string]string{"ephemeral": "true"}), IsNil)
	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "tenant1.test")

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker volume rm tenant1/test"), IsNil)
	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Not(Equals), "tenant1.test")

	c.Assert(s.createVolume("mon0", "tenant1", "test", map[string]string{"ephemeral": "false"}), IsNil)
	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "tenant1.test")

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker volume rm tenant1/test"), IsNil)
	out, err = s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "tenant1.test")
}

func (s *systemtestSuite) TestVolumeParallelCreate(c *C) {
	c.Assert(s.vagrant.IterateNodes(func(node utils.TestbedNode) error {
		return node.RunCommand("docker volume create -d volplugin --name tenant1/test")
	}), IsNil)

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "tenant1.test")

	c.Assert(s.vagrant.IterateNodes(func(node utils.TestbedNode) error {
		return node.RunCommand("docker volume rm tenant1/test")
	}), IsNil)
}
