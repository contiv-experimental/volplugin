package systemtests

import (
	"encoding/json"
	"time"

	"github.com/contiv/volplugin/config"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
)

func (s *systemtestSuite) TestVolpluginFDLeak(c *C) {
	log.Info("Running 2000 iterations of `docker volume ls` to ensure no FD exhaustion")
	c.Assert(s.vagrant.GetNode("mon0").RunCommand("set -e; for i in $(seq 0 2000); do docker volume ls; done"), IsNil)
}

func (s *systemtestSuite) TestVolpluginCrashRestart(c *C) {
	c.Assert(s.createVolume("mon0", "tenant1", "test", nil), IsNil)
	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker run -itd -v tenant1/test:/mnt alpine sleep 10m"), IsNil)
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	time.Sleep(10 * time.Second) // this is based on a 5s ttl set at volmaster/volplugin startup
	c.Assert(startVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	time.Sleep(1 * time.Second)
	c.Assert(s.createVolume("mon1", "tenant1", "test", nil), IsNil)
	c.Assert(s.vagrant.GetNode("mon1").RunCommand("docker run -itd -v tenant1/test:/mnt alpine sleep 10m"), NotNil)

	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(startVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	time.Sleep(10 * time.Second)
	c.Assert(s.createVolume("mon1", "tenant1", "test", nil), IsNil)
	c.Assert(s.vagrant.GetNode("mon1").RunCommand("docker run -itd -v tenant1/test:/mnt alpine sleep 10m"), NotNil)

	s.clearContainers()

	c.Assert(s.createVolume("mon1", "tenant1", "test", nil), IsNil)
	c.Assert(s.vagrant.GetNode("mon1").RunCommand("docker run -itd -v tenant1/test:/mnt alpine sleep 10m"), IsNil)
}

func (s *systemtestSuite) TestVolpluginHostLabel(c *C) {
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)

	c.Assert(s.vagrant.GetNode("mon0").RunCommandBackground("sudo -E `which volplugin` --host-label quux --debug --ttl 5"), IsNil)

	time.Sleep(10 * time.Millisecond)
	c.Assert(s.createVolume("mon0", "tenant1", "foo", nil), IsNil)

	out, err := s.docker("run -d -v tenant1/foo:/mnt alpine sleep 10m")
	c.Assert(err, IsNil)

	defer s.purgeVolume("mon0", "tenant1", "foo", true)
	defer s.docker("rm -f " + out)

	ut := &config.UseConfig{}

	// we know the pool is rbd here, so cheat a little.
	out, err = s.volcli("use get tenant1/foo")
	c.Assert(err, IsNil)
	c.Assert(json.Unmarshal([]byte(out), ut), IsNil)
	c.Assert(ut.Hostname, Equals, "quux")
}
