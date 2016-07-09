package systemtests

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/contiv/volplugin/config"
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestAPIServerFailedFormat(c *C) {
	if !cephDriver() {
		c.Skip("Only ceph supports filesystem formatting")
		return
	}

	_, err := s.uploadIntent("policy2", "fs")
	c.Assert(err, IsNil)

	volName := fqVolume("policy2", genRandomVolume())

	c.Assert(s.createVolume("mon0", volName, map[string]string{"filesystem": "falsefs"}), NotNil)
	_, err = s.volcli("volume remove " + volName)
	c.Assert(err, NotNil)
}

func (s *systemtestSuite) TestAPIServerGlobalConfigUpdate(c *C) {
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

func (s *systemtestSuite) TestAPIServerMultiRemove(c *C) {
	if !cephDriver() {
		c.Skip("Only ceph driver supports CRUD operations")
		return
	}

	volName := fqVolume("policy1", genRandomVolume())

	c.Assert(s.createVolume("mon0", volName, nil), IsNil)

	type out struct {
		out string
		err error
	}

	outChan := make(chan out, 5)

	for i := 0; i < 5; i++ {
		go func() {
			myout, err := s.volcli("volume remove " + volName)
			outChan <- out{myout, err}
		}()
	}

	errs := 0

	for i := 0; i < 5; i++ {
		myout := <-outChan
		if myout.err != nil {
			if myout.out != "" {
				c.Assert(strings.Contains(myout.out, fmt.Sprintf(`Error: Volume %s no longer exists`, volName)), Equals, true, Commentf("%v %v", myout.out, myout.err))
			}
			errs++
		}
	}

	c.Assert(errs, Equals, 4)
	c.Assert(s.purgeVolume("mon0", volName), NotNil)
}
