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

	do = DriverOptions{Timeout: 1, Volume: Volume{Name: "hi", Params: DriverParams{}}}
	c.Assert(do.Validate(), IsNil)
	do.Timeout = 0
	c.Assert(do.Validate(), NotNil)
	do.Timeout = 1
	c.Assert(do.Validate(), IsNil)
}

func (s *storageSuite) TestVolumeValidate(c *C) {
	v := Volume{}
	c.Assert(v.Validate(), NotNil)

	v = Volume{Name: "name", Size: 100, Params: DriverParams{}}
	c.Assert(v.Validate(), IsNil)
	v.Name = ""
	c.Assert(v.Validate(), NotNil)
	v.Name = "name"
	v.Params = nil
	c.Assert(v.Validate(), NotNil)
	v.Params = DriverParams{}
	c.Assert(v.Validate(), IsNil)
}

type testStruct struct {
	X int
	Y string
}

func (s *storageSuite) TestParams(c *C) {
	mp1 := DriverParams{"ceph": "1", "nfs": "1", "gluster": "1", "netapp": "0"}
	mp2 := DriverParams{"ceph": 1, "nfs": 1, "gluster": 1, "netapp": 2, "purestorage": 2}
	ts := DriverParams{"x": 1, "y": "string"}
	do := DriverOptions{Timeout: 1, Volume: Volume{Name: "TestParams", Params: DriverParams{"str": "testString", "int": 1, "mp1": mp1, "mp2": mp2, "struct": ts}}}

	// Integer type
	var intVal int
	err := do.Volume.Params.Get("int", &intVal)
	c.Assert(err, IsNil)
	c.Assert(intVal == 1, Equals, true)

	// Float type
	do.Volume.Params["float"] = 1.2
	var floatVal float64
	err = do.Volume.Params.Get("float", &floatVal)
	c.Assert(err, IsNil)
	c.Assert(floatVal == 1.2, Equals, true)

	// Bool type
	var boolVal bool
	do.Volume.Params["bool"] = true
	err = do.Volume.Params.Get("bool", &boolVal)
	c.Assert(err, IsNil)
	c.Assert(boolVal == true, Equals, true)

	// String type
	var strVal string
	err = do.Volume.Params.Get("str", &strVal)
	c.Assert(err, IsNil)
	c.Assert(strVal == "testString", Equals, true)

	// Map type
	var map1 map[string]string
	err = do.Volume.Params.Get("mp1", &map1)
	c.Assert(err, IsNil)
	c.Assert(len(map1) == len(mp1), Equals, true)
	c.Assert(map1["netapp"] == "0", Equals, true)

	var map2 map[string]int
	err = do.Volume.Params.Get("mp2", &map2)
	c.Assert(err, IsNil)
	c.Assert(len(map2) == len(mp2), Equals, true)
	c.Assert(map2["purestorage"] == 2, Equals, true)

	// Struct type
	tStruct := testStruct{}
	err = do.Volume.Params.Get("struct", &tStruct)
	c.Assert(err, IsNil)
	c.Assert(tStruct.X == 1, Equals, true)
	c.Assert(tStruct.Y == "string", Equals, true)

	// Below are few invalid cases!

	// Attr not found
	var x int
	err = do.Volume.Params.Get("x", &x)
	c.Assert(err, IsNil)
	c.Assert(x == 0, Equals, true) // default int value

	var mp map[string]string
	err = do.Volume.Params.Get("mp", &mp)
	c.Assert(err, IsNil)
	c.Assert(len(mp) == 0, Equals, true) // empty map

	// Type mismatches
	var str int
	err = do.Volume.Params.Get("str", &str) // trying to get string value into an integer
	c.Assert(err, NotNil)
	c.Assert(str == 0, Equals, true)

	// Marshaling should fail as its trying to pack map[string]string to string
	var map3 string
	err = do.Volume.Params.Get("mp1", &map3)
	c.Assert(err, NotNil)
	c.Assert(map3 == "", Equals, true)

	var bVal bool // capture float in bool type
	err = do.Volume.Params.Get("float", &bVal)
	c.Assert(err, NotNil)
	c.Assert(bVal == false, Equals, true) // default bool value

	// Nil values
	do.Volume.Params["nilValue"] = nil
	var nilV string
	err = do.Volume.Params.Get("nilValue", &nilV)
	c.Assert(err, NotNil) // typeOf(nilValue) == nil; error

	err = do.Volume.Params.Get("str", nil) // does not accept <nil> values as pointers
	c.Assert(err, NotNil)
}
