package systemtests

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/contiv/volplugin/config"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
)

func (s *systemtestSuite) TestVolpluginNoGlobalConfiguration(c *C) {
	_, err := s.mon0cmd("etcdctl rm /volplugin/global-config")
	c.Assert(err, IsNil)

	volName := fqVolume("policy1", genRandomVolume())

	c.Assert(s.createVolume("mon0", volName, nil), IsNil)
	out, err := s.dockerRun("mon0", false, false, volName, "echo")
	c.Assert(err, IsNil, Commentf(out))
}

func (s *systemtestSuite) TestVolpluginLockFreeOperation(c *C) {
	if !nfsDriver() {
		c.Skip("Cannot run this test on any driver but NFS")
		return
	}

	volName := fqVolume("policy1", genRandomVolume())

	out, err := s.uploadIntent("policy1", "unlocked")
	c.Assert(err, IsNil, Commentf(out))
	c.Assert(s.createVolume("mon0", volName, nil), IsNil)

	out, err = s.dockerRun("mon0", false, true, volName, "sleep 10m")
	c.Assert(err, IsNil, Commentf(out))

	out, err = s.dockerRun("mon1", false, true, volName, "sleep 10m")
	c.Assert(err, IsNil, Commentf(out))

	out, err = s.dockerRun("mon2", false, true, volName, "sleep 10m")
	c.Assert(err, IsNil, Commentf(out))
}

func (s *systemtestSuite) TestVolpluginAPIServerDown(c *C) {
	c.Assert(stopAPIServer(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(startVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(startAPIServer(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(s.createVolume("mon0", fqVolume("policy1", genRandomVolume()), nil), IsNil)
}

func (s *systemtestSuite) TestVolpluginCleanupSocket(c *C) {
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	defer c.Assert(startVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	_, err := s.mon0cmd("test -f /run/docker/plugins/volplugin.sock")
	c.Assert(err, NotNil)
}

func (s *systemtestSuite) TestVolpluginFDLeak(c *C) {
	c.Assert(s.restartNetplugin(), IsNil)
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
	if !cephDriver() {
		c.Skip("only ceph supports runtime parameters")
		return
	}

	volName := fqVolume("policy1", genRandomVolume())

	c.Assert(s.createVolume("mon0", volName, nil), IsNil)
	_, err := s.dockerRun("mon0", false, true, volName, "sleep 10m")
	c.Assert(err, IsNil)
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	time.Sleep(5 * time.Second)
	c.Assert(startVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(waitForVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	_, err = s.dockerRun("mon1", false, true, volName, "sleep 10m")
	c.Assert(err, NotNil)

	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(startVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	c.Assert(waitForVolplugin(s.vagrant.GetNode("mon0")), IsNil)

	_, err = s.volcli(fmt.Sprintf("volume runtime upload %s < /testdata/iops1.json", volName))
	c.Assert(err, IsNil)
	time.Sleep(45 * time.Second)
	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("sudo cat /sys/fs/cgroup/blkio/blkio.throttle.write_iops_device")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Not(Equals), "")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var found bool
	for _, line := range lines {
		parts := strings.Split(line, " ")
		c.Assert(len(parts), Equals, 2)
		if parts[1] == "1000" {
			found = true
		}
	}
	c.Assert(found, Equals, true)

	_, err = s.dockerRun("mon1", false, true, volName, "sleep 10m")
	c.Assert(err, NotNil)

	c.Assert(s.clearContainers(), IsNil)

	out, err = s.dockerRun("mon1", false, true, volName, "sleep 10m")
	c.Assert(err, IsNil, Commentf(out))
}

func (s *systemtestSuite) TestVolpluginHostLabel(c *C) {
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)

	c.Assert(s.vagrant.GetNode("mon0").RunCommandBackground("sudo -E `which volplugin` --host-label quux"), IsNil)

	volName := fqVolume("policy1", genRandomVolume())

	time.Sleep(10 * time.Millisecond)
	c.Assert(s.createVolume("mon0", volName, nil), IsNil)

	out, err := s.dockerRun("mon0", false, true, volName, "sleep 10m")
	c.Assert(err, IsNil)

	defer s.purgeVolume("mon0", volName)
	defer s.mon0cmd("docker rm -f " + out)

	ut := &config.UseMount{}

	// we know the pool is rbd here, so cheat a little.
	out, err = s.volcli("use get " + volName)
	c.Assert(err, IsNil, Commentf(out))
	c.Assert(json.Unmarshal([]byte(out), ut), IsNil, Commentf(out))
	c.Assert(ut.Hostname, Equals, "quux")
}

func (s *systemtestSuite) TestVolpluginMountPath(c *C) {
	if !cephDriver() {
		c.Skip("Only ceph driver has mounts that work like this (for now)")
		return
	}

	c.Assert(s.uploadGlobal("mountpath_global"), IsNil)
	time.Sleep(time.Second)
	volName := genRandomVolume()
	c.Assert(s.createVolume("mon0", fqVolume("policy1", volName), nil), IsNil)
	_, err := s.dockerRun("mon0", false, true, fqVolume("policy1", volName), "sleep 10m")
	c.Assert(err, IsNil)

	c.Assert(s.vagrant.GetNode("mon0").RunCommand("sudo test -d /mnt/test/rbd/policy1."+volName), IsNil)
}

func (s *systemtestSuite) TestVolpluginRestartMultiMount(c *C) {
	_, err := s.mon0cmd("sudo truncate -s0 /tmp/volplugin.log")
	c.Assert(err, IsNil)

	volName := fqVolume("policy1", genRandomVolume())

	c.Assert(s.createVolume("mon0", volName, map[string]string{"unlocked": "true"}), IsNil)
	out, err := s.dockerRun("mon0", false, true, volName, "sleep 10")
	c.Assert(err, IsNil)
	out2, err := s.dockerRun("mon0", false, true, volName, "sleep 10")
	c.Assert(err, IsNil)
	c.Assert(stopVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	time.Sleep(100 * time.Millisecond)
	c.Assert(startVolplugin(s.vagrant.GetNode("mon0")), IsNil)
	time.Sleep(100 * time.Millisecond)

	out = strings.TrimSpace(out)
	out2 = strings.TrimSpace(out2)

	errout, err := s.mon0cmd(fmt.Sprintf("docker kill -s KILL '%s' && sleep 1 && docker rm '%s'", out, out))
	c.Assert(err, IsNil, Commentf(errout))
	errout, err = s.mon0cmd(fmt.Sprintf("docker kill -s KILL '%s' && sleep 1 && docker rm '%s'", out2, out2))
	c.Assert(err, IsNil, Commentf(errout))

	errout, err = s.mon0cmd("grep 500 /tmp/volplugin.log")
	c.Assert(err, NotNil, Commentf(errout))
}
