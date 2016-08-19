package systemtests

import (
	"strings"

	"github.com/contiv/remotessh"

	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestVolumeCreateMultiHostCrossHostMount(c *C) {
	if nullDriver() {
		c.Skip("This driver does not support multi-host operation")
		return
	}

	fqVolName := fqVolume("policy1", genRandomVolume())

	c.Assert(s.createVolume("mon0", fqVolName, nil), IsNil)

	_, err := s.dockerRun("mon0", false, false, fqVolName, `sh -c "echo bar > /mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(s.createVolume("mon1", fqVolName, nil), IsNil)

	out, err := s.dockerRun("mon1", false, false, fqVolName, `sh -c "cat /mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "bar")

	c.Assert(s.createVolume("mon1", fqVolName, nil), IsNil)

	_, err = s.dockerRun("mon1", false, false, fqVolName, `sh -c "echo quux > /mnt/foo"`)
	c.Assert(err, IsNil)

	c.Assert(s.createVolume("mon2", fqVolName, nil), IsNil)

	out, err = s.dockerRun("mon2", false, false, fqVolName, `sh -c "cat /mnt/foo"`)
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "quux")
}

func (s *systemtestSuite) TestVolumeMultiPolicyCreate(c *C) {
	_, err := s.uploadIntent("policy2", "policy2")
	c.Assert(err, IsNil)

	fqVolName := fqVolume("policy1", genRandomVolume())
	fqVolName2 := fqVolume("policy2", genRandomVolume())

	c.Assert(s.createVolume("mon0", fqVolName, nil), IsNil)
	c.Assert(s.createVolume("mon0", fqVolName2, nil), IsNil)

	_, err = s.dockerRun("mon0", false, false, fqVolName, `sh -c "echo foo > /mnt/bar"`)
	c.Assert(err, IsNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.dockerRun("mon0", false, false, fqVolName2, `sh -c "cat /mnt/bar"`)
	c.Assert(err, NotNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.dockerRun("mon0", false, false, fqVolName2, `sh -c "echo bar > /mnt/foo"`)
	c.Assert(err, IsNil)

	c.Assert(s.clearContainers(), IsNil)

	_, err = s.dockerRun("mon0", false, false, fqVolName, `sh -c "cat /mnt/foo"`)
	c.Assert(err, NotNil)
}

func (s *systemtestSuite) TestVolumeMultiCreateThroughDocker(c *C) {
	if !cephDriver() {
		c.Skip("This is only supported by the ceph driver")
		return
	}

	volName := genRandomVolume()
	fqVolName := fqVolume("policy1", volName)

	c.Assert(s.createVolume("mon0", fqVolName, nil), IsNil)

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo rbd ls")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "policy1."+volName)

	c.Assert(s.vagrant.IterateNodes(func(node remotessh.TestbedNode) error {
		return node.RunCommand("docker volume create -d volcontiv --name " + fqVolName)
	}), IsNil)

	c.Assert(s.vagrant.IterateNodes(func(node remotessh.TestbedNode) error {
		return node.RunCommand("docker volume rm " + fqVolName)
	}), IsNil)
}
