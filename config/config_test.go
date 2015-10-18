package config

import (
	"os/exec"
	"path"
	. "testing"
	"time"

	. "gopkg.in/check.v1"
)

type configSuite struct {
	tlc *TopLevelConfig
}

var _ = Suite(&configSuite{})

func TestConfig(t *T) { TestingT(t) }

func (s *configSuite) SetUpTest(c *C) {
	cmd := exec.Command("/bin/sh", "-c", "sudo systemctl start etcd")
	c.Assert(cmd.Run(), IsNil)
	time.Sleep(200 * time.Millisecond)
}

func (s *configSuite) TearDownTest(c *C) {
	cmd := exec.Command("/bin/sh", "-c", "pkill etcd; rm -rf /var/lib/etcd")
	c.Assert(cmd.Run(), IsNil)
	time.Sleep(200 * time.Millisecond)
}

func (s *configSuite) SetUpSuite(c *C) {
	s.tlc = NewTopLevelConfig("/volplugin", []string{"http://127.0.0.1:2379"})
}

func (s *configSuite) TestPrefixed(c *C) {
	c.Assert(s.tlc.prefixed("foo"), Equals, path.Join(s.tlc.prefix, "foo"))
	c.Assert(s.tlc.mount("bar", "baz"), Equals, s.tlc.prefixed(rootMount, "bar", "baz"))
	c.Assert(s.tlc.tenant("quux"), Equals, s.tlc.prefixed(rootTenant, "quux"))
	c.Assert(s.tlc.volume("foo", "bar"), Equals, s.tlc.prefixed(rootVolume, "foo", "bar"))
}
