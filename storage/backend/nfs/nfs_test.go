package nfs

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	. "testing"

	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/mountscan"

	. "gopkg.in/check.v1"
)

const (
	rootPath  = "/volplugin-nfs"
	mountPath = "/mnt"
)

type nfsSuite struct{}

var _ = Suite(&nfsSuite{})

func TestNFS(t *T) { TestingT(t) }

func umountAll(c *C) {
	c.Assert(exec.Command("umount", "-t", "nfs4", "-a").Run(), IsNil)
}

func (s *nfsSuite) SetUpSuite(c *C) {
	umountAll(c)
	resetExports(c)
}

func (s *nfsSuite) TearDownSuite(c *C) {
	umountAll(c)
	resetExports(c)
}

func (s *nfsSuite) SetUpTest(c *C) {
	umountAll(c)

	// ensure that we don't trip over nonexistence errors, but also handle other
	// errors if they arrive.
	if err := os.RemoveAll(rootPath); !os.IsNotExist(err) {
		c.Assert(err, IsNil)
	}

	resetExports(c)
}

func (s *nfsSuite) TearDownTest(c *C) {
}

func runExports(c *C) {
	c.Assert(exec.Command("exportfs", "-a").Run(), IsNil)
}

func resetExports(c *C) {
	if err := os.RemoveAll("/etc/exports.d"); !os.IsNotExist(err) {
		c.Assert(err, IsNil)
	}

	c.Assert(os.Mkdir("/etc/exports.d", 0777), IsNil)
	runExports(c)
}

func mkPath(name string) string {
	return path.Join(rootPath, name)
}

func mkTargetPath(name string) string {
	return path.Join(mountPath, name)
}

func nfsMount(name string) string {
	return fmt.Sprintf("localhost:%s", mkPath(name))
}

func makeExport(c *C, name, mountArgs string) {
	c.Assert(ioutil.WriteFile(fmt.Sprintf("/etc/exports.d/%s.exports", name), []byte(strings.Join([]string{mkPath(name), fmt.Sprintf("*(%s)\n", mountArgs)}, " ")), 0644), IsNil)
	c.Assert(os.MkdirAll(mkPath(name), 0777), IsNil)
	runExports(c)
}

func (s *nfsSuite) TestSanity(c *C) {
	d, err := NewMountDriver(mountPath)
	c.Assert(err, IsNil)

	c.Assert(d.Name(), Equals, BackendName)
	c.Assert(BackendName, Equals, "nfs")
}

func (s *nfsSuite) TestRepeatedMountSingleMountPoint(c *C) {
	// XXX I haven't quite figured out what hte maximum threshold for our stock
	// nfs server configuration is yet. I do know however that >500 is going to
	// result in many I/O errors as NFS gets overloaded. I'm not sure if this is
	// just a limitation of the nfsd per-mount or what. Configuring the threadpool
	// doesn't seem to help.
	for i := 0; i < 100; i++ {
		resetExports(c)

		if err := os.RemoveAll(rootPath); !os.IsNotExist(err) {
			c.Assert(err, IsNil)
		}

		makeExport(c, "basic", "rw")

		c.Assert(ioutil.WriteFile(path.Join(mkPath("basic"), "foo"), []byte{byte(i)}, 0644), IsNil)
		d, err := NewMountDriver(mountPath)
		c.Assert(err, IsNil)

		vol := storage.Volume{
			Name: "test1/basic",
		}

		do := storage.DriverOptions{
			Source: nfsMount("basic"),
			Volume: vol,
		}

		m, err := d.Mount(do)
		c.Assert(err, IsNil)
		c.Assert(m, DeepEquals, &storage.Mount{
			Device: nfsMount("basic"),
			Path:   mkTargetPath("test1/basic"),
			Volume: vol,
		})

		_, err = os.Stat(path.Join(mkTargetPath("test1/basic"), "foo"))
		c.Assert(err, IsNil)

		c.Assert(d.Unmount(do), IsNil)
	}
}

func (s *nfsSuite) TestNFSOptionsFromString(c *C) {
	d := &Driver{mountpath: mountPath}
	m, err := d.validateConvertOptions("")
	c.Assert(err, IsNil)
	c.Assert(m, DeepEquals, map[string]string{})

	invalid := []string{
		",",
		"=",
		",part",
		"part,",
		"part=",
		"part=,",
		",foo=bar",
		"foo=bar,",
		"foo=bar=baz",
		"foo=bar,=baz",
	}

	for _, str := range invalid {
		m, err = d.validateConvertOptions(str)
		c.Assert(err, NotNil, Commentf(str))
		c.Assert(m, IsNil, Commentf(str))
	}

	valid := map[string]map[string]string{
		"foo=bar":                 {"foo": "bar"},
		"foo=bar,baz=quux":        {"foo": "bar", "baz": "quux"},
		"spork,comma=text,market": {"spork": "", "comma": "text", "market": ""},
	}

	for str, val := range valid {
		m, err := d.validateConvertOptions(str)
		c.Assert(err, IsNil, Commentf("%#v - %v", str, err))
		c.Assert(m, DeepEquals, val, Commentf("%#v - %v", str, err))
	}
}

func (s *nfsSuite) TestNFSOptionsFromDriverOptions(c *C) {
	d := &Driver{mountpath: mountPath}

	invalidSources := []string{
		"300.300.300.300:/mnt",
		"/mnt",
		"300.300.300.300",
		"300.300.300.300",
	}

	invalidOptions := []string{
		",",
		"=",
		",part",
		"part,",
		"part=",
		"part=,",
		",foo=bar",
		"foo=bar,",
		"foo=bar=baz",
		"foo=bar,=baz",
	}

	for _, source := range invalidSources {
		do := storage.DriverOptions{
			Source: source,
			Volume: storage.Volume{
				Name: "foo/bar",
				Params: storage.DriverParams{
					"options": "test=1",
				},
			},
		}

		str, err := d.mkOpts(do)
		c.Assert(err, NotNil, Commentf("%#v %v", do, str))
		c.Assert(str, Equals, "")
	}

	for _, options := range invalidOptions {
		do := storage.DriverOptions{
			Source: "localhost:/mnt",
			Volume: storage.Volume{
				Name: "foo/bar",
				Params: storage.DriverParams{
					"options": options,
				},
			},
		}

		str, err := d.mkOpts(do)
		c.Assert(err, NotNil)
		c.Assert(str, Equals, "")
	}

	do := storage.DriverOptions{
		Source: "localhost:/mnt",
		Volume: storage.Volume{
			Name: "foo/bar",
			Params: storage.DriverParams{
				"options": "rw,sync,test=1",
			},
		},
	}

	str, err := d.mkOpts(do)
	c.Assert(err, IsNil)
	m, err := d.validateConvertOptions(str)
	c.Assert(err, IsNil)
	c.Assert(m, DeepEquals, map[string]string{
		"nfsvers":    "4",
		"sync":       "",
		"rw":         "",
		"test":       "1",
		"addr":       "::1",
		"clientaddr": "::1",
	})
}

func (s *nfsSuite) TestMountScan(c *C) {
	resetExports(c)
	if err := os.RemoveAll(rootPath); !os.IsNotExist(err) {
		c.Assert(err, IsNil)
	}

	makeExport(c, "mountscan", "rw")
	mountD, err := NewMountDriver(mountPath)
	c.Assert(err, IsNil)

	do := storage.DriverOptions{
		Source: nfsMount("mountscan"),
		Volume: storage.Volume{
			Name: "test/mountscan",
		},
	}
	_, err = mountD.Mount(do)
	c.Assert(err, IsNil)

	driver := &Driver{}
	name, err := driver.internalName(do.Volume.Name)
	c.Assert(err, IsNil)

	hostMounts, err := mountscan.GetMounts(&mountscan.GetMountsRequest{DriverName: "nfs", FsType: "nfs4"})
	c.Assert(err, IsNil)

	found := false
	for _, hostMount := range hostMounts {
		if found = strings.Contains(hostMount.MountPoint, name); found {
			break
		}
	}
	c.Assert(mountD.Unmount(do), IsNil)
	c.Assert(found, Equals, true)
}
