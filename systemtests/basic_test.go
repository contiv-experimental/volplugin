package systemtests

import . "gopkg.in/check.v1"

func (s *systemtestSuite) TestBasicStarted(c *C) {
	c.Assert(s.vagrant.GetNode("mon0").RunCommand("pgrep -c apiserver"), IsNil)
	c.Assert(s.vagrant.SSHExecAllNodes("pgrep -c volplugin"), IsNil)
}

func (s *systemtestSuite) TestBasicSSH(c *C) {
	c.Assert(s.vagrant.SSHExecAllNodes("/bin/echo"), IsNil)
}
