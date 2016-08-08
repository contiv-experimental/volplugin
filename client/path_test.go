package client

import (
	. "testing"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"

	. "gopkg.in/check.v1"
)

type pathSuite struct{}

var _ = Suite(&pathSuite{})

func TestPath(t *T) { TestingT(t) }

func (s *pathSuite) TestPather(c *C) {
	consulPather := NewConsulPather("volplugin")
	etcdPather := NewEtcdPather("volplugin")

	path, err := consulPather.Replace("test", "quux")
	c.Assert(err, IsNil)
	c.Assert(path.String(), Equals, "test/quux")
	path, err = consulPather.Append("volplugin", "test", "quux")
	c.Assert(err, IsNil)

	inter, err := consulPather.Append("test", "quux")
	c.Assert(err, IsNil)

	path2, err := consulPather.Combine(inter)
	c.Assert(err, IsNil)
	c.Assert(path, DeepEquals, path2)

	path, err = etcdPather.Replace("test", "quux")
	c.Assert(err, IsNil)
	c.Assert(path.String(), Equals, "/test/quux")
	path, err = etcdPather.Append("test", "quux")
	c.Assert(err, IsNil)
	inter, err = etcdPather.Replace("test", "quux")
	c.Assert(err, IsNil)

	path2, err = etcdPather.Combine(inter)
	c.Assert(err, IsNil)
	c.Assert(path, DeepEquals, path2)
}

func (s *pathSuite) TestPatherRestrictedCharacters(c *C) {
	consulPather := NewConsulPather("volplugin")
	etcdPather := NewEtcdPather("volplugin")

	for _, pather := range []*Pather{consulPather, etcdPather} {
		path, err := pather.Append("some/content")
		c.Assert(err, NotNil, Commentf("%v", path))
		c.Assert(err.(*errored.Error).Contains(errors.InvalidPath), Equals, true)
		path, err = pather.Replace("some/content")
		c.Assert(err, NotNil, Commentf("%v", path))
		c.Assert(err.(*errored.Error).Contains(errors.InvalidPath), Equals, true)

		// cheating. I don't think this is actually possible to do.
		p2 := *pather
		p2.content = []string{"axl&/"}
		path, err = pather.Combine(&p2)
		c.Assert(err, NotNil, Commentf("%v", path))
		c.Assert(err.(*errored.Error).Contains(errors.InvalidPath), Equals, true)
	}
}
