package test

import (
	"github.com/contiv/volplugin/db"
	. "gopkg.in/check.v1"
)

func (s *testSuite) TestGlobal(c *C) {
	global := db.NewGlobal()
	c.Assert(s.client.Set(global), IsNil)
	global2 := db.NewGlobal()
	c.Assert(s.client.Get(global2), IsNil)
	global = global.Canonical()
	c.Assert(global, DeepEquals, global2)
}

func (s *testSuite) TestGlobalEmpty(c *C) {
	c.Assert(s.client.Set(db.NewGlobal()), IsNil)
	global := db.NewGlobal()
	c.Assert(s.client.Get(global), IsNil)

	c.Assert(global.TTL, Equals, db.DefaultGlobalTTL)
	c.Assert(global.MountPath, Equals, db.DefaultMountPath)
	c.Assert(global.Timeout, Equals, db.DefaultTimeout)
}

func (s *testSuite) TestGlobalWatch(c *C) {
	global := db.NewGlobal()

	globalChan, errChan := s.client.Watch(global)
	c.Assert(s.client.Set(global), IsNil)
	select {
	case err := <-errChan:
		c.Assert(err, IsNil) // will always fail
	case tmp := (<-globalChan):
		global2 := tmp.(*db.Global)
		c.Assert(global2, NotNil)
		global2 = global2.Canonical()
		c.Assert(global, DeepEquals, global2)
	}

	c.Assert(s.client.WatchStop(global), IsNil)
}
