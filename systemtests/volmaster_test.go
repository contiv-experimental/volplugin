package systemtests

import (
	"io/ioutil"
	"strings"

	"github.com/contiv/volplugin/config"
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestVolmasterNoGlobalConfiguration(c *C) {
	c.Assert(s.vagrant.IterateNodes(stopVolmaster), IsNil)
	_, err := s.mon0cmd("etcdctl rm /volplugin/global-config")
	c.Assert(err, IsNil)
	c.Assert(s.vagrant.IterateNodes(startVolmaster), IsNil)
	c.Assert(s.vagrant.IterateNodes(waitForVolmaster), IsNil)

	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)
	out, err := s.dockerRun("mon0", false, false, "policy1/test", "echo")
	c.Assert(err, IsNil, Commentf(out))
}

func (s *systemtestSuite) TestVolmasterFailedFormat(c *C) {
	if !cephDriver() {
		c.Skip("Only ceph supports filesystem formatting")
		return
	}

	_, err := s.uploadIntent("policy2", "fs")
	c.Assert(err, IsNil)
	c.Assert(s.createVolume("mon0", "policy2", "testfalse", map[string]string{"filesystem": "falsefs"}), NotNil)
	_, err = s.volcli("volume remove policy2/testfalse")
	c.Assert(err, NotNil)
}

func (s *systemtestSuite) TestVolmasterGlobalConfigUpdate(c *C) {
	content, err := ioutil.ReadFile("testdata/globals/global1.json")
	c.Assert(err, IsNil)

	globalBase1, err := config.NewGlobalConfigFromJSON(content)
	c.Assert(err, IsNil)

	content, err = ioutil.ReadFile("testdata/globals/global2.json")
	c.Assert(err, IsNil)

	globalBase2, err := config.NewGlobalConfigFromJSON(content)
	c.Assert(err, IsNil)

	c.Assert(s.uploadGlobal("global1"), IsNil)

	out, err := s.volcli("global get")
	c.Assert(err, IsNil)

	global, err := config.NewGlobalConfigFromJSON([]byte(out))
	c.Assert(err, IsNil)

	c.Assert(globalBase1, DeepEquals, global)
	c.Assert(globalBase2, Not(DeepEquals), global)

	c.Assert(s.uploadGlobal("global2"), IsNil)

	out, err = s.volcli("global get")
	c.Assert(err, IsNil)

	global, err = config.NewGlobalConfigFromJSON([]byte(out))
	c.Assert(err, IsNil)

	c.Assert(globalBase1, Not(DeepEquals), global)
	c.Assert(globalBase2, DeepEquals, global)
}

func (s *systemtestSuite) TestVolmasterMultiRemove(c *C) {
	if !cephDriver() {
		c.Skip("Only ceph driver supports CRUD operations")
		return
	}

	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)

	type out struct {
		out string
		err error
	}

	outChan := make(chan out, 5)

	for i := 0; i < 5; i++ {
		go func() {
			myout, err := s.volcli("volume remove policy1/test")
			outChan <- out{myout, err}
		}()
	}

	errs := 0

	for i := 0; i < 5; i++ {
		myout := <-outChan
		if myout.err != nil {
			if myout.out != "" {
				c.Assert(strings.Contains(myout.out, `Error: Volume policy1/test no longer exists`), Equals, true, Commentf("%v %v", myout.out, myout.err))
			}
			errs++
		}
	}

	c.Assert(errs, Equals, 4)
	c.Assert(s.purgeVolume("mon0", "policy1", "test", true), NotNil)
}
