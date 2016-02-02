package systemtests

import (
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestVolmasterFailedFormat(c *C) {
	_, err := s.uploadIntent("tenant2", "fs")
	c.Assert(s.createVolume("mon0", "tenant2", "testfalse", map[string]string{"filesystem": "falsefs"}), NotNil)
	_, err = s.volcli("volume remove tenant2/testfalse")
	c.Assert(err, IsNil)
}
