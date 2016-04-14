package ceph

import (
	"io"
	"os"
	"os/exec"
	"strings"
	. "testing"
	"time"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/storage"
)

const myMountpath = "/mnt/ceph"

var filesystems = map[string]storage.FSOptions{
	"ext4": {
		Type:          "ext4",
		CreateCommand: "mkfs.ext4 -m0 %",
	},
}

var volumeSpec = storage.Volume{
	Name:   "test/pithos",
	Size:   10,
	Params: storage.Params{"pool": "rbd"},
}

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

func (s *cephSuite) TestMkfsVolume(c *C) {
	// Create a new driver; the ceph driver is needed
	driver := Driver{mountpath: myMountpath}

	err := driver.mkfsVolume("echo %s; sleep 1", "fake-fake-fake", 3*time.Second)
	c.Assert(err, IsNil)

	err = driver.mkfsVolume("echo %s; sleep 2", "fake-fake-fake", 1*time.Second)
	c.Assert(err, NotNil)
}

func (s *cephSuite) TestMountUnmountVolume(c *C) {
	// Create a new driver
	crudDriver, err := NewCRUDDriver()
	c.Assert(err, IsNil)
	mountDriver, err := NewMountDriver(myMountpath)
	c.Assert(err, IsNil)

	driverOpts := storage.DriverOptions{
		Volume:    volumeSpec,
		FSOptions: filesystems["ext4"],
		Timeout:   5 * time.Second,
	}

	// we don't care if there's an error here, just want to make sure the create
	// succeeds. Easier restart of failed tests this way.
	mountDriver.Unmount(driverOpts)
	crudDriver.Destroy(driverOpts)

	c.Assert(crudDriver.Create(driverOpts), IsNil)
	defer crudDriver.Destroy(driverOpts)
	c.Assert(crudDriver.Format(driverOpts), IsNil)
	ms, err := mountDriver.Mount(driverOpts)
	c.Assert(err, IsNil)
	c.Assert(ms.Volume, DeepEquals, volumeSpec)
	c.Assert(ms.DevMajor, Equals, uint(252))
	c.Assert(ms.DevMinor, Equals, uint(0))
	c.Assert(strings.HasPrefix(ms.Device, "/dev/rbd"), Equals, true)
	mp, err := mountDriver.MountPath(driverOpts)
	c.Assert(err, IsNil)
	s.readWriteTest(c, mp)
	c.Assert(mountDriver.Unmount(driverOpts), IsNil)
	c.Assert(crudDriver.Destroy(driverOpts), IsNil)
}

func (s *cephSuite) TestSnapshots(c *C) {
	snapDrv, err := NewSnapshotDriver()
	c.Assert(err, IsNil)
	crudDrv, err := NewCRUDDriver()
	c.Assert(err, IsNil)

	driverOpts := storage.DriverOptions{
		Volume:    volumeSpec,
		FSOptions: filesystems["ext4"],
		Timeout:   5 * time.Second,
	}

	c.Assert(crudDrv.Create(driverOpts), IsNil)
	defer crudDrv.Destroy(driverOpts)
	c.Assert(snapDrv.CreateSnapshot("hello", driverOpts), IsNil)
	c.Assert(snapDrv.CreateSnapshot("hello", driverOpts), NotNil)

	list, err := snapDrv.ListSnapshots(driverOpts)
	c.Assert(err, IsNil)
	c.Assert(len(list), Equals, 1)
	c.Assert(list, DeepEquals, []string{"hello"})

	c.Assert(snapDrv.RemoveSnapshot("hello", driverOpts), IsNil)
	c.Assert(snapDrv.RemoveSnapshot("hello", driverOpts), NotNil)

	list, err = snapDrv.ListSnapshots(driverOpts)
	c.Assert(err, IsNil)
	c.Assert(len(list), Equals, 0)
	c.Assert(crudDrv.Destroy(driverOpts), IsNil)
}

func (s *cephSuite) TestRepeatedMountUnmount(c *C) {
	mountDrv, err := NewMountDriver(myMountpath)
	c.Assert(err, IsNil)
	crudDrv, err := NewCRUDDriver()
	c.Assert(err, IsNil)

	driverOpts := storage.DriverOptions{
		Volume:    volumeSpec,
		FSOptions: filesystems["ext4"],
		Timeout:   5 * time.Second,
	}

	// we don't care if there's an error here, just want to make sure the create
	// succeeds. Easier restart of failed tests this way.
	mountDrv.Unmount(driverOpts)
	crudDrv.Destroy(driverOpts)

	defer mountDrv.Unmount(driverOpts)
	defer crudDrv.Destroy(driverOpts)

	c.Assert(crudDrv.Create(driverOpts), IsNil)
	c.Assert(crudDrv.Format(driverOpts), IsNil)
	for i := 0; i < 10; i++ {
		_, err := mountDrv.Mount(driverOpts)
		c.Assert(err, IsNil)
		s.readWriteTest(c, "/mnt/ceph/rbd/test.pithos")
		c.Assert(mountDrv.Unmount(driverOpts), IsNil)
	}
	c.Assert(crudDrv.Destroy(driverOpts), IsNil)
}

func (s *cephSuite) TestTemplateFSCmd(c *C) {
	c.Assert(templateFSCmd("%", "foo"), Equals, "foo")
	c.Assert(templateFSCmd("%%", "foo"), Equals, "%%")
	c.Assert(templateFSCmd("%%%", "foo"), Equals, "%%foo")
	c.Assert(templateFSCmd("% test % test %", "foo"), Equals, "foo test foo test foo")
	c.Assert(templateFSCmd("% %% %", "foo"), Equals, "foo %% foo")
	c.Assert(templateFSCmd("mkfs.ext4 -m0 %", "/dev/sda1"), Equals, "mkfs.ext4 -m0 /dev/sda1")
}

func (s *cephSuite) TestMounted(c *C) {
	crudDrv, err := NewCRUDDriver()
	c.Assert(err, IsNil)
	mountDrv, err := NewMountDriver(myMountpath)
	c.Assert(err, IsNil)

	driverOpts := storage.DriverOptions{
		Volume:    volumeSpec,
		FSOptions: filesystems["ext4"],
		Timeout:   2 * time.Minute,
	}

	// we don't care if there's an error here, just want to make sure the create
	// succeeds. Easier restart of failed tests this way.
	mountDrv.Unmount(driverOpts)
	crudDrv.Destroy(driverOpts)

	c.Assert(crudDrv.Create(driverOpts), IsNil)
	c.Assert(crudDrv.Format(driverOpts), IsNil)
	_, err = mountDrv.Mount(driverOpts)
	c.Assert(err, IsNil)
	mounts, err := mountDrv.Mounted(2 * time.Minute)
	c.Assert(err, IsNil)

	intName, err := (&Driver{}).internalName(volumeSpec.Name) // totally cheating
	c.Assert(err, IsNil)

	c.Assert(*mounts[0], DeepEquals, storage.Mount{
		Device:   "/dev/rbd0",
		DevMajor: 252,
		DevMinor: 0,
		Path:     strings.Join([]string{myMountpath, volumeSpec.Params["pool"], intName}, "/"),
		Volume: storage.Volume{
			Name: "test/pithos",
			Params: map[string]string{
				"pool": "rbd",
			},
		},
	})

	c.Assert(mountDrv.Unmount(driverOpts), IsNil)
	c.Assert(crudDrv.Destroy(driverOpts), IsNil)
}

func (s *cephSuite) TestExternalInternalNames(c *C) {
	driver := &Driver{}
	out, err := driver.internalName("tenant1/test")
	c.Assert(err, IsNil)
	c.Assert(out, Equals, "tenant1.test")

	out, err = driver.internalName("tenant1.test/test")
	c.Assert(err, NotNil)
	c.Assert(out, Equals, "")

	out, err = driver.internalName("tenant1/test.two")
	c.Assert(err, IsNil)
	c.Assert(out, Equals, "tenant1.test.two")

	out, err = driver.internalName("tenant1/test/two")
	c.Assert(err, NotNil)
	c.Assert(out, Equals, "")

	out, err = driver.internalName("tenant1/test")
	c.Assert(driver.externalName(out), Equals, "tenant1/test")
}

func (s *cephSuite) TestSnapshotClone(c *C) {
	snapDrv, err := NewSnapshotDriver()
	c.Assert(err, IsNil)
	crudDrv, err := NewCRUDDriver()
	c.Assert(err, IsNil)

	driverOpts := storage.DriverOptions{
		Volume:    volumeSpec,
		FSOptions: filesystems["ext4"],
		Timeout:   5 * time.Second,
	}

	c.Assert(crudDrv.Create(driverOpts), IsNil)
	c.Assert(snapDrv.CreateSnapshot("test", driverOpts), IsNil)
	c.Assert(snapDrv.CopySnapshot(driverOpts, "test", "test/image"), IsNil)
	defer func(driverOpts storage.DriverOptions) {
		driverOpts.Volume.Name = "test/image"
		c.Assert(crudDrv.Destroy(driverOpts), IsNil)
		c.Assert(exec.Command("rbd", "snap", "unprotect", "test.pithos", "--snap", "test", "--pool", volumeSpec.Params["pool"]).Run(), IsNil)
		driverOpts.Volume.Name = "test/pithos"
		c.Assert(crudDrv.Destroy(driverOpts), IsNil)
	}(driverOpts)

	content, err := exec.Command("rbd", "ls").CombinedOutput()
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(string(content)), Equals, "test.image\ntest.pithos")
	c.Assert(snapDrv.CopySnapshot(driverOpts, "foo", "test/image"), NotNil)
	c.Assert(snapDrv.CopySnapshot(driverOpts, "test", "test/image"), NotNil)
}
