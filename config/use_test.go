package config

import (
	"sort"
	"time"

	"github.com/coreos/etcd/client"

	. "gopkg.in/check.v1"
)

var testUseVolumes = map[string]*Volume{
	"basic":  {PolicyName: "policy1", VolumeName: "quux"},
	"basic2": {PolicyName: "policy2", VolumeName: "baz"},
}

var testUseMounts = map[string]*UseMount{
	"basic": {
		Volume:   testUseVolumes["basic"],
		Hostname: "hostname",
	},
	"basic2": {
		Volume:   testUseVolumes["basic2"],
		Hostname: "hostname",
	},
}

func (s *configSuite) TestUseCRUD(c *C) {
	c.Assert(s.tlc.PublishUse(testUseMounts["basic"]), IsNil)
	c.Assert(s.tlc.PublishUse(testUseMounts["basic"]), NotNil)
	c.Assert(s.tlc.RemoveUse(testUseMounts["basic"], false), IsNil)
	c.Assert(s.tlc.PublishUse(testUseMounts["basic"]), IsNil)

	mt := &UseMount{}

	c.Assert(s.tlc.GetUse(mt, testUseVolumes["basic"]), IsNil)
	c.Assert(testUseMounts["basic"], DeepEquals, mt)

	c.Assert(s.tlc.PublishUse(testUseMounts["basic2"]), IsNil)
	c.Assert(s.tlc.PublishUse(testUseMounts["basic2"]), NotNil)
	c.Assert(s.tlc.RemoveUse(testUseMounts["basic2"], false), IsNil)
	c.Assert(s.tlc.PublishUse(testUseMounts["basic2"]), IsNil)

	c.Assert(s.tlc.GetUse(mt, testUseVolumes["basic2"]), IsNil)
	c.Assert(testUseMounts["basic2"], DeepEquals, mt)

	mounts, err := s.tlc.ListUses("mount")
	c.Assert(err, IsNil)

	sort.Strings(mounts)
	c.Assert([]string{"policy1/quux", "policy2/baz"}, DeepEquals, mounts)

	basicTmp := *testUseMounts["basic"]
	basicTmp.Hostname = "quux"

	c.Assert(s.tlc.RemoveUse(&basicTmp, false), NotNil)
	c.Assert(s.tlc.RemoveUse(&basicTmp, true), IsNil)
}

func (s *configSuite) TestUseCRUDWithTTL(c *C) {
	c.Assert(s.tlc.PublishUseWithTTL(testUseMounts["basic"], 5*time.Second, client.PrevNoExist), IsNil)
	use := &UseMount{}
	c.Assert(s.tlc.GetUse(use, testUseVolumes["basic"]), IsNil)
	c.Assert(use, DeepEquals, testUseMounts["basic"])
	time.Sleep(10 * time.Second)
	c.Assert(s.tlc.GetUse(use, testUseVolumes["basic"]), NotNil)

	c.Assert(s.tlc.PublishUseWithTTL(testUseMounts["basic"], 5*time.Second, client.PrevNoExist), IsNil)
	c.Assert(s.tlc.PublishUseWithTTL(testUseMounts["basic"], 5*time.Second, client.PrevExist), IsNil)
	c.Assert(s.tlc.PublishUseWithTTL(testUseMounts["basic2"], 5*time.Second, client.PrevNoExist), IsNil)
}

func (s *configSuite) TestUseListEtcdDown(c *C) {
	stopStartEtcd(c, func() {
		_, err := s.tlc.ListUses("mount")
		c.Assert(err, NotNil)
	})
}
