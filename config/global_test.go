package config

import . "gopkg.in/check.v1"

func (s *configSuite) TestGlobal(c *C) {
	_, err := s.tlc.GetGlobal()
	c.Assert(err, NotNil)

	global := &Global{
		Debug:   true,
		TTL:     10,
		Timeout: 1,
	}

	c.Assert(s.tlc.PublishGlobal(global), IsNil)
	global2, err := s.tlc.GetGlobal()
	c.Assert(err, IsNil)
	c.Assert(global, DeepEquals, DivideGlobalParameters(global2))
}

func (s *configSuite) TestGlobalWatch(c *C) {
	activity := make(chan *Global)

	global := &Global{
		Debug:   true,
		TTL:     10,
		Timeout: 1,
	}

	// XXX this leaks but w/e, we should probably implement a stop chan. not a
	// real world problem
	go s.tlc.WatchGlobal(activity)

	c.Assert(s.tlc.PublishGlobal(global), IsNil)
	global2 := DivideGlobalParameters(<-activity)
	c.Assert(global2, NotNil)
	c.Assert(global, DeepEquals, global2)
}
