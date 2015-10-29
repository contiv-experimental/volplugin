package cephdriver

import (
	"io"
	"os"
	"strings"
	. "testing"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
)

type cephSuite struct{}

var _ = Suite(&cephSuite{})

func TestCeph(t *T) { TestingT(t) }

func (s *cephSuite) SetUpTest(c *C) {
	if os.Getenv("DEBUG") != "" {
		log.SetLevel(log.DebugLevel)
	}
}

func (s *cephSuite) readWriteTest(c *C, mountDir string) {
	// Write a file and verify you can read it
	file, err := os.Create(mountDir + "/test.txt")
	c.Assert(err, IsNil)

	_, err = file.WriteString("Test string\n")
	c.Assert(err, IsNil)

	file.Close()

	file, err = os.Open(mountDir + "/test.txt")
	c.Assert(err, IsNil)

	rb := make([]byte, 11)
	_, err = io.ReadAtLeast(file, rb, 11)
	c.Assert(err, IsNil)

	file.Close()

	var rbs = strings.TrimSpace(string(rb))
	c.Assert(rbs, Equals, strings.TrimSpace("Test string"))
}

func (s *cephSuite) TestMountUnmountVolume(c *C) {
	// Create a new driver
	volumeSpec := NewCephDriver().NewVolume("rbd", "pithos1234", 10)

	// we don't care if there's an error here, just want to make sure the create
	// succeeds. Easier restart of failed tests this way.
	volumeSpec.Unmount()
	volumeSpec.Remove()

	c.Assert(volumeSpec.Create("mkfs.ext4 -m0 %"), IsNil)
	ms, err := volumeSpec.Mount("ext4")
	c.Assert(err, IsNil)
	c.Assert(ms.DevMajor, Equals, uint(252))
	c.Assert(ms.DevMinor, Equals, uint(0))
	c.Assert(strings.HasPrefix(ms.DeviceName, "/dev/rbd"), Equals, true)
	s.readWriteTest(c, "/mnt/ceph/rbd/pithos1234")
	c.Assert(volumeSpec.Unmount(), IsNil)
	c.Assert(volumeSpec.Remove(), IsNil)
}

func (s *cephSuite) TestSnapshots(c *C) {
	volumeSpec := NewCephDriver().NewVolume("rbd", "pithos1234", 10)
	c.Assert(volumeSpec.Create("mkfs.ext4 -m0 %"), IsNil)
	defer volumeSpec.Remove()
	c.Assert(volumeSpec.CreateSnapshot("hello"), IsNil)
	c.Assert(volumeSpec.CreateSnapshot("hello"), NotNil)

	list, err := volumeSpec.ListSnapshots()
	c.Assert(err, IsNil)
	c.Assert(len(list), Equals, 1)
	c.Assert(list, DeepEquals, []string{"hello"})

	c.Assert(volumeSpec.RemoveSnapshot("hello"), IsNil)
	c.Assert(volumeSpec.RemoveSnapshot("hello"), NotNil)

	list, err = volumeSpec.ListSnapshots()
	c.Assert(err, IsNil)
	c.Assert(len(list), Equals, 0)
	c.Assert(volumeSpec.Remove(), IsNil)
}

func (s *cephSuite) TestRepeatedMountUnmount(c *C) {
	volumeSpec := NewCephDriver().NewVolume("rbd", "pithos1234", 10)
	c.Assert(volumeSpec.Create("mkfs.ext4 -m0 %"), IsNil)
	for i := 0; i < 10; i++ {
		_, err := volumeSpec.Mount("ext4")
		c.Assert(err, IsNil)
		s.readWriteTest(c, "/mnt/ceph/rbd/pithos1234")
		c.Assert(volumeSpec.Unmount(), IsNil)
	}
	c.Assert(volumeSpec.Remove(), IsNil)
}

func (s *cephSuite) TestTemplateFSCmd(c *C) {
	c.Assert(templateFSCmd("%", "foo"), Equals, "foo")
	c.Assert(templateFSCmd("%%", "foo"), Equals, "%%")
	c.Assert(templateFSCmd("%%%", "foo"), Equals, "%%foo")
	c.Assert(templateFSCmd("% test % test %", "foo"), Equals, "foo test foo test foo")
	c.Assert(templateFSCmd("% %% %", "foo"), Equals, "foo %% foo")
	c.Assert(templateFSCmd("mkfs.ext4 -m0 %", "/dev/sda1"), Equals, "mkfs.ext4 -m0 /dev/sda1")
}
