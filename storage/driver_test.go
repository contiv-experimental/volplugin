package storage

import (
	. "testing"

	. "gopkg.in/check.v1"
)

type storageSuite struct{}

var _ = Suite(&storageSuite{})

func TestStorage(t *T) { TestingT(t) }

func (s *storageSuite) TestDriverOptionsValidate(c *C) {
	do := DriverOptions{}

	c.Assert(do.Validate(), NotNil)

	do = DriverOptions{Timeout: 1, Volume: Volume{Name: "hi", Params: map[string]string{}}}
	c.Assert(do.Validate(), IsNil)
	do.Timeout = 0
	c.Assert(do.Validate(), NotNil)
	do.Timeout = 1
	c.Assert(do.Validate(), IsNil)
}

func (s *storageSuite) TestVolumeValidate(c *C) {
	v := Volume{}
	c.Assert(v.Validate(), NotNil)

	v = Volume{Name: "name", Size: 100, Params: map[string]string{}}
	c.Assert(v.Validate(), IsNil)
	v.Name = ""
	c.Assert(v.Validate(), NotNil)
	v.Name = "name"
	v.Params = nil
	c.Assert(v.Validate(), NotNil)
	v.Params = map[string]string{}
	c.Assert(v.Validate(), IsNil)
}
