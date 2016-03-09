package systemtests

import (
	"encoding/json"
	"time"

	"github.com/contiv/volplugin/config"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
)

func (s *systemtestSuite) TestVolpluginCleanupSocket(c *C) {
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	defer c.Assert(startVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	_, err := s.mon0cmd("test -f /run/docker/plugins/volplugin.sock")
	c.Assert(err, NotNil)
}

func (s *systemtestSuite) TestVolpluginFDLeak(c *C) {
	iterations := 2000
	subIterations := 50

	log.Infof("Running %d iterations of `docker volume ls` to ensure no FD exhaustion", iterations)

	errChan := make(chan error, iterations)

	for i := 0; i < iterations/subIterations; i++ {
		go func() {
			for i := 0; i < subIterations; i++ {
				errChan <- s.vagrant.GetNode("mon0").RunCommand("docker volume ls")
			}
		}()
	}

	for i := 0; i < iterations; i++ {
		c.Assert(<-errChan, IsNil)
	}
}

func (s *systemtestSuite) TestVolpluginCrashRestart(c *C) {
	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)
	c.Assert(s.vagrant.GetNode("mon0").RunCommand("docker run -itd -v policy1/test:/mnt alpine sleep 10m"), IsNil)
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	time.Sleep(10 * time.Second) // this is based on a 5s ttl set at volmaster/volplugin startup
	c.Assert(startVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	time.Sleep(1 * time.Second)
	c.Assert(s.createVolume("mon1", "policy1", "test", nil), IsNil)
	c.Assert(s.vagrant.GetNode("mon1").RunCommand("docker run -itd -v policy1/test:/mnt alpine sleep 10m"), NotNil)

	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(startVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	time.Sleep(10 * time.Second)
	c.Assert(s.createVolume("mon1", "policy1", "test", nil), IsNil)
	c.Assert(s.vagrant.GetNode("mon1").RunCommand("docker run -itd -v policy1/test:/mnt alpine sleep 10m"), NotNil)

	s.clearContainers()

	c.Assert(s.createVolume("mon1", "policy1", "test", nil), IsNil)
	c.Assert(s.vagrant.GetNode("mon1").RunCommand("docker run -itd -v policy1/test:/mnt alpine sleep 10m"), IsNil)
}

func (s *systemtestSuite) TestVolpluginHostLabel(c *C) {
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)

	c.Assert(s.vagrant.GetNode("mon0").RunCommandBackground("sudo -E `which volplugin` --host-label quux"), IsNil)

	time.Sleep(10 * time.Millisecond)
	c.Assert(s.createVolume("mon0", "policy1", "foo", nil), IsNil)

	out, err := s.docker("run -d -v policy1/foo:/mnt alpine sleep 10m")
	c.Assert(err, IsNil)

	defer s.purgeVolume("mon0", "policy1", "foo", true)
	defer s.docker("rm -f " + out)

	ut := &config.UseMount{}

	// we know the pool is rbd here, so cheat a little.
	out, err = s.volcli("use get policy1/foo")
	c.Assert(err, IsNil)
	c.Assert(json.Unmarshal([]byte(out), ut), IsNil)
	c.Assert(ut.Hostname, Equals, "quux")
}

func (s *systemtestSuite) TestVolpluginMountPath(c *C) {
	c.Assert(s.uploadGlobal("mountpath_global"), IsNil)
	time.Sleep(1 * time.Second)
	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)
	_, err := s.docker("run -d -v policy1/test:/mnt alpine sleep 10m")
	c.Assert(err, IsNil)
	c.Assert(s.vagrant.GetNode("mon0").RunCommand("sudo test -d /mnt/test/rbd/policy1.test"), IsNil)
}
