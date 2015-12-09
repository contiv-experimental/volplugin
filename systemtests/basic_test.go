package systemtests

import . "gopkg.in/check.v1"

func (s *systemtestSuite) TestBasicStarted(c *C) {
	c.Assert(s.vagrant.GetNode("mon0").RunCommand("pgrep -c volmaster"), IsNil)
	c.Assert(s.vagrant.SSHAllNodes("pgrep -c volplugin"), IsNil)
}

func (s *systemtestSuite) TestBasicSSH(c *C) {
	c.Assert(s.vagrant.SSHAllNodes("/bin/echo"), IsNil)
}
