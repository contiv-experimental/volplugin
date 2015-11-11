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
	exec.Command("/bin/sh", "-c", "etcdctl rm --recursive /volplugin").Run()
}

func (s *configSuite) SetUpSuite(c *C) {
	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl start etcd").Run(), IsNil)
	time.Sleep(200 * time.Millisecond)

	tlc, err := NewTopLevelConfig("/volplugin", []string{"http://127.0.0.1:2379"})
	if err != nil {
		c.Fatal(err)
	}

	s.tlc = tlc
}

func (s *configSuite) TestPrefixed(c *C) {
	c.Assert(s.tlc.prefixed("foo"), Equals, path.Join(s.tlc.prefix, "foo"))
	c.Assert(s.tlc.use(&VolumeConfig{TenantName: "bar", VolumeName: "baz"}), Equals, s.tlc.prefixed(rootUse, "bar", "baz"))
	c.Assert(s.tlc.tenant("quux"), Equals, s.tlc.prefixed(rootTenant, "quux"))
	c.Assert(s.tlc.volume("foo", "bar"), Equals, s.tlc.prefixed(rootVolume, "foo", "bar"))
}
