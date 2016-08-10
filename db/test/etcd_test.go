package test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/contiv/executor"
	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/impl/etcd"
	"golang.org/x/net/context"
)

func (s *testSuite) TestNewClient(c *C) {
	client, err := getEtcdClient()
	c.Assert(err, IsNil)

	volClient, err := etcd.NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil)
	c.Assert(volClient, NotNil)

	_, err = client.Get(context.Background(), "/volplugin", nil)
	c.Assert(err, IsNil)

	volClient, err = etcd.NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil, Commentf("%v", err))
	c.Assert(volClient, NotNil)

	volClient, err = etcd.NewClient(etcdHosts, "testing")
	c.Assert(err, IsNil)
	c.Assert(volClient, NotNil)

	_, err = client.Get(context.Background(), "/testing", nil)
	c.Assert(err, IsNil)
}

func (s *testSuite) TestDump(c *C) {
	volClient, err := etcd.NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil)

	key := "/volplugin/foo"
	value := "baz"

	// add test key to etcd
	result, err := executor.New(exec.Command("/bin/sh", "-c", "etcdctl set "+key+" "+value)).Run(context.Background())
	c.Assert(err, IsNil)
	c.Assert(result.ExitStatus, Equals, 0)

	// remove test key after the test completes
	defer func() {
		result, err := executor.New(exec.Command("/bin/sh", "-c", "etcdctl rm "+key)).Run(context.Background())
		c.Assert(err, IsNil)
		c.Assert(result.ExitStatus, Equals, 0)
	}()

	tarballPath, err := volClient.Dump("")
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

func (s *testSuite) TestCRUD(c *C) {
	volClient, err := etcd.NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil)

	client, err := getEtcdClient()
	c.Assert(err, IsNil)

	te := newTestEntity("test", "data")
	path, err := te.Path()
	c.Assert(path, Equals, "test/test")
	c.Assert(volClient.Get(te), NotNil)

	c.Assert(volClient.Set(te), IsNil)

	te = newTestEntity("test", "")
	c.Assert(volClient.Get(te), IsNil)
	c.Assert(te.SomeData, Equals, "data")

	_, err = client.Get(context.Background(), "/volplugin/test/test", nil)
	c.Assert(err, IsNil)

	entities, err := volClient.List(te)
	c.Assert(err, IsNil)
	c.Assert(len(entities), Equals, 1)
	c.Assert(entities[0].(*testEntity).Name, Equals, "test")
	c.Assert(entities[0].(*testEntity).SomeData, Equals, "data")

	c.Assert(volClient.Delete(te), IsNil)
	c.Assert(volClient.Get(te), NotNil)
}

func (s *testSuite) TestWatch(c *C) {
	volClient, err := etcd.NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil)

	te := newTestEntity("test", "")

	channel := make(chan interface{}, 10)

	retChan, errChan := volClient.Watch(te)

	for i := 0; i < 10; i++ {
		te2 := te.Copy().(*testEntity)
		te2.SomeData = fmt.Sprintf("data%d", i)
		setKey(te2)
	}

	for i := 0; i < 10; i++ {
		select {
		case err := <-errChan:
			c.Assert(err, IsNil, Commentf("select: %v", err)) // this will always fail, assert is just to raise the error.
		case ent := <-retChan:
			te, ok := ent.(*testEntity)
			c.Assert(ok, Equals, true)
			c.Assert(te, NotNil)
			c.Assert(te.Name, Equals, "test")
			c.Assert(string(te.SomeData), Equals, fmt.Sprintf("data%d", i))
		}
	}

	c.Assert(volClient.WatchStop(te), IsNil)
	for i := 0; i < 10; i++ {
		te2 := te.Copy().(*testEntity)
		te2.SomeData = fmt.Sprintf("data%d", i)
		setKey(te2)
	}

	var x = 0

	for i := 0; i < 10; i++ {
		select {
		case <-channel:
			x++
		default:
		}
	}
}

func (s *testSuite) TestHooks(c *C) {
	volClient, err := etcd.NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil)

	triggered := &struct{ i int }{i: 0}

	te := newTestEntity("test", "")
	triggerFunc := func(c db.Client, te db.Entity) error {
		triggered.i++
		return nil
	}

	te.hooks.PreSet = triggerFunc
	c.Assert(volClient.Set(te), IsNil)
	c.Assert(triggered.i, Equals, 1)

	triggered.i = 0
	te.hooks.PostSet = triggerFunc
	c.Assert(volClient.Set(te), IsNil)
	c.Assert(triggered.i, Equals, 2)

	triggered.i = 0
	te.hooks.PreSet = nil
	te.hooks.PostSet = nil
	c.Assert(volClient.Set(te), IsNil)
	c.Assert(triggered.i, Equals, 0)

	te.hooks.PreGet = triggerFunc
	c.Assert(volClient.Get(te), IsNil)
	c.Assert(triggered.i, Equals, 1)

	triggered.i = 0
	te.hooks.PostGet = triggerFunc
	c.Assert(volClient.Get(te), IsNil)
	c.Assert(triggered.i, Equals, 2)

	triggered.i = 0
	te.hooks.PreGet = nil
	te.hooks.PostGet = nil
	c.Assert(volClient.Get(te), IsNil)
	c.Assert(triggered.i, Equals, 0)

	te.hooks.PreDelete = triggerFunc
	c.Assert(volClient.Delete(te), IsNil)
	c.Assert(triggered.i, Equals, 1)

	// to test these delete calls, we have to re-set the data.
	c.Assert(volClient.Set(te), IsNil)

	triggered.i = 0
	te.hooks.PostDelete = triggerFunc
	c.Assert(volClient.Delete(te), IsNil)
	c.Assert(triggered.i, Equals, 2)

	// to test these delete calls, we have to re-set the data.
	c.Assert(volClient.Set(te), IsNil)

	triggered.i = 0
	te.hooks.PreDelete = nil
	te.hooks.PostDelete = nil
	c.Assert(volClient.Delete(te), IsNil)
	c.Assert(triggered.i, Equals, 0)
}
