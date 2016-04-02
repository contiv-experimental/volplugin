package config

import (
	"os/exec"
	"path"
	. "testing"

	. "gopkg.in/check.v1"
)

type configSuite struct {
	tlc *Client
}

var _ = Suite(&configSuite{})

func TestConfig(t *T) { TestingT(t) }

func (s *configSuite) SetUpTest(c *C) {
	exec.Command("/bin/sh", "-c", "etcdctl rm --recursive /volplugin").Run()
}

func (s *configSuite) SetUpSuite(c *C) {
	tlc, err := NewClient("/volplugin", []string{"http://127.0.0.1:2379"})
	if err != nil {
		c.Fatal(err)
	}

	s.tlc = tlc
}

func (s *configSuite) TestPrefixed(c *C) {
	c.Assert(s.tlc.prefixed("foo"), Equals, path.Join(s.tlc.prefix, "foo"))
	c.Assert(s.tlc.use("mount", &Volume{PolicyName: "bar", VolumeName: "baz"}), Equals, s.tlc.prefixed(rootUse, "mount", "bar", "baz"))
	c.Assert(s.tlc.policy("quux"), Equals, s.tlc.prefixed(rootPolicy, "quux"))
	c.Assert(s.tlc.volume("foo", "bar", "quux"), Equals, s.tlc.prefixed(rootVolume, "foo", "bar", "quux"))
}
