package test

import (
	"github.com/contiv/volplugin/db"
	. "gopkg.in/check.v1"
)

func (s *testSuite) TestRuntimeWatch(c *C) {
	c.Assert(s.client.Set(testPolicies["basic"]), IsNil)
	policy := db.NewPolicy("basic")
	c.Assert(s.client.Get(policy), IsNil)

	vol, err := db.CreateVolume(&db.VolumeRequest{Policy: policy, Name: "bar"})
	c.Assert(err, IsNil)

	objChan, errChan := s.client.WatchPrefix(&db.RuntimeOptions{})
	defer s.client.WatchPrefixStop(&db.RuntimeOptions{})
	opts := db.NewRuntimeOptions(vol.PolicyName, vol.VolumeName)
	opts.RateLimit.ReadBPS = 1000
	c.Assert(s.client.Set(opts), IsNil)

	select {
	case err := <-errChan:
		c.Assert(err, IsNil)
	case obj := <-objChan:
		c.Assert(obj.(*db.RuntimeOptions).RateLimit.ReadBPS, Equals, uint64(1000))
		c.Assert(obj.(*db.RuntimeOptions).Policy(), Not(Equals), "")
		c.Assert(obj.(*db.RuntimeOptions).Policy(), Equals, vol.PolicyName)
		c.Assert(obj.(*db.RuntimeOptions).Volume(), Not(Equals), "")
		c.Assert(obj.(*db.RuntimeOptions).Volume(), Equals, vol.VolumeName)
	}
}
