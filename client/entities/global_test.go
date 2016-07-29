package entities

import (
	"encoding/json"
	"time"

	"golang.org/x/net/context"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"

	. "gopkg.in/check.v1"
)

func (s *entitySuite) TestGlobal(c *C) {
	c.Assert(NewGlobal().setEmpty(), DeepEquals, &Global{
		TTL:       DefaultGlobalTTL,
		MountPath: defaultMountPath,
		Timeout:   DefaultTimeout,
	})
}

func (s *entitySuite) TestGlobalClient(c *C) {
	gc := NewGlobalClient(s.client)

	c.Assert(gc.Path(), Equals, gc.client.Path())

	// most operations on globals currently take empty strings as arguments,
	// since there is only one path to select, but this allows them to still
	// conform to the interface.
	_, err := gc.Get("")
	c.Assert(err, NotNil)
	c.Assert(err.(*errored.Error).Contains(errors.GetGlobal), Equals, true)

	c.Assert(gc.Publish(NewGlobal()), IsNil)

	path, err := gc.Path().Replace("global-config")
	c.Assert(err, IsNil)

	val, err := s.client.Get(context.Background(), path, nil)
	c.Assert(err, IsNil)

	g := &Global{}
	c.Assert(json.Unmarshal(val.Value, g), IsNil)
	c.Assert(g, DeepEquals, NewGlobal())

	g.TTL = 100 * time.Hour // absurdly large ttl to inject variance
	c.Assert(gc.Publish(g), IsNil)
	g2, err := gc.Get("")
	c.Assert(err, IsNil)
	c.Assert(g, DeepEquals, g2)
	c.Assert(gc.Delete(""), IsNil)
	g2, err = gc.Get("")
	c.Assert(err, NotNil)
	c.Assert(g2, IsNil)
}

func (s *entitySuite) TestGlobalClientWatch(c *C) {
	gc := NewGlobalClient(s.client)

	channel, errChan, err := gc.Watch("")
	c.Assert(err, IsNil)

	for i := 0; i < 10; i++ {
		payload, err := NewGlobal().Payload()
		c.Assert(err, IsNil)
		path, err := gc.Path().Replace("global-config")
		c.Assert(err, IsNil)
		s.setKey(path, payload)
	}

	for i := 0; i < 10; i++ {
		select {
		case err := <-errChan:
			c.Assert(err, IsNil, Commentf("select: %v", err)) // this will always fail, assert is just to raise the error.
		case intf := <-channel:
			node, ok := intf.(*Global)
			c.Assert(ok, Equals, true)
			c.Assert(node, NotNil)
		}
	}

	gc.WatchStop("")

	for i := 0; i < 10; i++ {
		payload, err := NewGlobal().Payload()
		c.Assert(err, IsNil)
		path, err := gc.Path().Replace("global-config")
		c.Assert(err, IsNil)
		s.setKey(path, payload)
	}

	var x = 0

	for i := 0; i < 10; i++ {
		select {
		case <-channel:
			x++
		default:
		}
	}

	c.Assert(x, Equals, 0)
}
