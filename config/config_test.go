package config

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	. "testing"

	. "gopkg.in/check.v1"

	"github.com/contiv/executor"
	"golang.org/x/net/context"
)

type configSuite struct {
	tlc *Client
}

var _ = Suite(&configSuite{})

func TestConfig(t *T) { TestingT(t) }

func (s *configSuite) SetUpTest(c *C) {
	exec.Command("/bin/sh", "-c", "etcdctl rm --recursive /volplugin").Run()
}

func (s *configSuite) SetUpSuite(c *C) {
	tlc, err := NewClient("/volplugin", []string{"http://127.0.0.1:2379"})
	if err != nil {
		c.Fatal(err)
	}

	s.tlc = tlc
}

func (s *configSuite) TestPrefixed(c *C) {
	c.Assert(s.tlc.prefixed("foo"), Equals, path.Join(s.tlc.prefix, "foo"))
	c.Assert(s.tlc.use("mount", "bar/baz"), Equals, s.tlc.prefixed(rootUse, "mount", "bar", "baz"))
	c.Assert(s.tlc.policy("quux"), Equals, s.tlc.prefixed(rootPolicy, "quux"))
	c.Assert(s.tlc.volume("foo", "bar", "quux"), Equals, s.tlc.prefixed(rootVolume, "foo", "bar", "quux"))
}

func (s *configSuite) TestDumpTarball(c *C) {
	key := "/volplugin/foo"
	value := "baz"

	// add test key to etcd
	e := executor.New(exec.Command("/bin/sh", "-c", "etcdctl set "+key+" "+value))
	e.Start()
	result, err := e.Wait(context.Background())
	c.Assert(err, IsNil)
	c.Assert(result.ExitStatus, Equals, 0)

	// remove test key after the test completes
	defer func() {
		e := executor.New(exec.Command("/bin/sh", "-c", "etcdctl rm "+key))
		e.Start()
		result, err := e.Wait(context.Background())
		c.Assert(err, IsNil)
		c.Assert(result.ExitStatus, Equals, 0)
	}()

	tarballPath, err := s.tlc.DumpTarball()
	c.Assert(err, IsNil)
	defer os.Remove(tarballPath)

	// check that the tarball was created in the expected spot
	_, err = os.Stat(tarballPath)
	c.Assert(os.IsNotExist(err), Equals, false)

	tarballFilename := filepath.Base(tarballPath)
	// can't use filepath.Ext() here because it thinks ".gz" is the extension
	dirName := tarballFilename[:strings.LastIndex(tarballFilename, ".tar.gz")]

	testFilename := dirName + key

	data, err := ioutil.ReadFile(tarballPath)
	c.Assert(err, IsNil)

	// check that our test key is present and has the expected value
	reader := bytes.NewReader(data)
	gzReader, err := gzip.NewReader(reader)
	c.Assert(err, IsNil)
	tarReader := tar.NewReader(gzReader)

	found := false

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		c.Assert(err, IsNil)

		if header.Name == testFilename {
			var b bytes.Buffer
			_, err = io.Copy(&b, tarReader)
			c.Assert(err, IsNil)
			c.Assert(string(b.Bytes()), Equals, value)
			found = true
			break
		}
	}

	c.Assert(found, Equals, true)
}
