package config

import (
	"os/exec"
	"path"
	. "testing"

	. "gopkg.in/check.v1"
)

type configSuite struct {
	tlc *TopLevelConfig
}

var _ = Suite(&configSuite{})

func TestConfig(t *T) { TestingT(t) }

func (s *configSuite) SetUpTest(c *C) {
	cmd := exec.Command("/bin/sh", "-c", "etcd -data-dir /tmp/etcd")
	c.Assert(cmd.Start(), IsNil)
}

func (s *configSuite) TearDownTest(c *C) {
	cmd := exec.Command("/bin/sh", "-c", "pkill etcd; rm -rf /tmp/etcd")
	c.Assert(cmd.Run(), IsNil)
}

func (s *configSuite) SetUpSuite(c *C) {
	s.tlc = NewTopLevelConfig("/volplugin", []string{"127.0.0.1:2379"})
}

func (s *configSuite) TestPrefixed(c *C) {
	c.Assert(s.tlc.prefixed("foo"), Equals, path.Join(s.tlc.prefix, "foo"))
	c.Assert(s.tlc.mount("bar", "baz"), Equals, s.tlc.prefixed(rootMount, "bar", "baz"))
	c.Assert(s.tlc.tenant("quux"), Equals, s.tlc.prefixed(rootTenant, "quux"))
	c.Assert(s.tlc.volume("foo", "bar"), Equals, s.tlc.prefixed(rootVolume, "foo", "bar"))
}
