package config

import (
	"github.com/contiv/volplugin/watch"
	. "gopkg.in/check.v1"
)

func (s *configSuite) TestGlobal(c *C) {
	_, err := s.tlc.GetGlobal()
	c.Assert(err, NotNil)

	global := &Global{
		Debug:     true,
		TTL:       DefaultGlobalTTL,
		Timeout:   DefaultTimeout,
		MountPath: defaultMountPath,
	}

	c.Assert(s.tlc.PublishGlobal(global), IsNil)
	global2, err := s.tlc.GetGlobal()
	c.Assert(err, IsNil)
	global = global.Canonical()
	c.Assert(global, DeepEquals, global2)
}

func (s *configSuite) TestGlobalEmpty(c *C) {
	c.Assert(s.tlc.PublishGlobal(&Global{}), IsNil)
	global, err := s.tlc.GetGlobal()
	c.Assert(err, IsNil)

	c.Assert(global.SetEmpty(), DeepEquals, &Global{
		TTL:       DefaultGlobalTTL,
		MountPath: defaultMountPath,
		Timeout:   DefaultTimeout,
	})
}

func (s *configSuite) TestGlobalWatch(c *C) {
	activity := make(chan *watch.Watch)

	global := &Global{
		Debug:     true,
		TTL:       DefaultGlobalTTL,
		Timeout:   DefaultTimeout,
		MountPath: defaultMountPath,
	}

	// XXX this leaks but w/e, we should probably implement a stop chan. not a
	// real world problem
	s.tlc.WatchGlobal(activity)

	c.Assert(s.tlc.PublishGlobal(global), IsNil)
	global2 := (<-activity).Config.(*Global)
	c.Assert(global2, NotNil)
	global2 = global2.Canonical()
	c.Assert(global, DeepEquals, global2)
}
