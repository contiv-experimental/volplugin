package systemtests

import (
	"encoding/json"
	"io/ioutil"

	"github.com/contiv/volplugin/config"
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestVolmasterNoGlobalConfiguration(c *C) {
	c.Assert(s.vagrant.IterateNodes(stopVolmaster), IsNil)
	_, err := s.mon0cmd("etcdctl rm /volplugin/global-config")
	c.Assert(err, IsNil)
	c.Assert(s.vagrant.IterateNodes(startVolmaster), IsNil)

	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)
	_, err = s.docker("run -v policy1/test:/mnt alpine echo")
	c.Assert(err, IsNil)
}

func (s *systemtestSuite) TestVolmasterFailedFormat(c *C) {
	_, err := s.uploadIntent("policy2", "fs")
	c.Assert(err, IsNil)
	c.Assert(s.createVolume("mon0", "policy2", "testfalse", map[string]string{"filesystem": "falsefs"}), NotNil)
	_, err = s.volcli("volume remove policy2/testfalse")
	c.Assert(err, IsNil)
}

func (s *systemtestSuite) TestVolmasterGlobalConfigUpdate(c *C) {
	content, err := ioutil.ReadFile("testdata/global1.json")
	c.Assert(err, IsNil)

	globalBase1 := &config.Global{}
	c.Assert(json.Unmarshal(content, globalBase1), IsNil)

	content, err = ioutil.ReadFile("testdata/global2.json")
	c.Assert(err, IsNil)

	globalBase2 := &config.Global{}
	c.Assert(json.Unmarshal(content, globalBase2), IsNil)

	out, err := s.volcli("global get")
	c.Assert(err, IsNil)

	global := &config.Global{}
	c.Assert(json.Unmarshal([]byte(out), global), IsNil)

	c.Assert(globalBase1, DeepEquals, global)
	c.Assert(globalBase2, Not(DeepEquals), global)

	c.Assert(s.uploadGlobal("global2"), IsNil)

	out, err = s.volcli("global get")
	c.Assert(err, IsNil)

	global = &config.Global{}
	c.Assert(json.Unmarshal([]byte(out), global), IsNil)

	c.Assert(globalBase1, Not(DeepEquals), global)
	c.Assert(globalBase2, DeepEquals, global)
}
