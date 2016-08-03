package mountscan

import (
	"os"
	"os/exec"

	. "testing"

	log "github.com/Sirupsen/logrus"
	. "gopkg.in/check.v1"
)

type mountscanSuite struct{}

var _ = Suite(&mountscanSuite{})

func TestMountscan(t *T) { TestingT(t) }

func (s *mountscanSuite) SetUpTest(c *C) {
	if os.Getenv("DEBUG") != "" {
		log.SetLevel(log.DebugLevel)
	}
}

func (s *mountscanSuite) TestGetMounts(c *C) {
	srcDir := "/tmp/src"
	targetDir := "/tmp/target"

	c.Assert(exec.Command("mkdir", "-p", srcDir).Run(), IsNil)
	c.Assert(exec.Command("mkdir", "-p", targetDir).Run(), IsNil)
	c.Assert(exec.Command("mount", "--bind", srcDir, targetDir).Run(), IsNil)

	hostMounts, err := GetMounts(&GetMountsRequest{DriverName: "none", KernelDriver: "device-mapper"})
	c.Assert(err, IsNil)

	found := false
	for _, hostMount := range hostMounts {
		if found = (hostMount.MountPoint == targetDir && hostMount.Root == srcDir); found {
			break
		}
	}

	c.Assert(exec.Command("umount", targetDir).Run(), IsNil)
	c.Assert(exec.Command("rm", "-r", targetDir).Run(), IsNil)
	c.Assert(exec.Command("rm", "-r", srcDir).Run(), IsNil)
	c.Assert(found, Equals, true)
}

func (s *mountscanSuite) TestGetMountsInput(c *C) {
	_, err := GetMounts(&GetMountsRequest{DriverName: "nfs", FsType: "nfs4"})
	c.Assert(err, IsNil)

	_, err = GetMounts(&GetMountsRequest{DriverName: "ceph", KernelDriver: "rbd"})
	c.Assert(err, IsNil)

	_, err = GetMounts(&GetMountsRequest{DriverName: "none", KernelDriver: "device-mapper"})
	c.Assert(err, IsNil)

	_, err = GetMounts(&GetMountsRequest{DriverName: ""})
	c.Assert(err, ErrorMatches, ".*DriverName is required.*")

	_, err = GetMounts(&GetMountsRequest{DriverName: "nfs"})
	c.Assert(err, ErrorMatches, ".*Filesystem type is required.*")

	_, err = GetMounts(&GetMountsRequest{DriverName: "ceph"})
	c.Assert(err, ErrorMatches, ".*Kernel driver is required.*")
}
