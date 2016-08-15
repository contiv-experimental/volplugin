package volmigrate

import (
	. "testing"

	. "gopkg.in/check.v1"
)

type volmigrateSuite struct {
}

var _ = Suite(&volmigrateSuite{})

func TestVolmigrate(t *T) { TestingT(t) }

func (s *volmigrateSuite) TestStuff(c *C) {
	c.Assert(nil, IsNil)
}
