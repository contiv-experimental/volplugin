package test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/jsonio"
)

func (s *testSuite) TestDump(c *C) {
	copy := testPolicies["basic"].Copy()
	for i := 0; i < 10; i++ {
		copy.(*db.Policy).Name = fmt.Sprintf("test%d", i)
		c.Assert(s.client.Set(copy), IsNil)
	}

	tarballPath, err := s.client.Dump("")
	c.Assert(err, IsNil)
	defer os.Remove(tarballPath)

	// check that the tarball was created in the expected spot
	_, err = os.Stat(tarballPath)
	c.Assert(os.IsNotExist(err), Equals, false)

	tarballFilename := filepath.Base(tarballPath)
	// can't use filepath.Ext() here because it thinks ".gz" is the extension
	dirName := tarballFilename[:strings.LastIndex(tarballFilename, ".tar.gz")]

	for i := 0; i < 10; i++ {
		testFilename := path.Join(dirName, "volplugin/policies", fmt.Sprintf("test%d", i))

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

				copy := testPolicies["basic"].Copy()
				copy.(*db.Policy).Name = fmt.Sprintf("test%d", i)
				c.Assert(s.client.Get(copy), IsNil)

				content, err := jsonio.Write(copy)
				c.Assert(err, IsNil)
				c.Assert(b.String(), Equals, string(content))
				found = true
				break
			}
		}

		c.Assert(found, Equals, true, Commentf(testFilename))
	}
}

func (s *testSuite) TestCRUD(c *C) {
	te := newTestEntity("test", "data")
	path, err := te.Path()
	c.Assert(err, IsNil)
	c.Assert(path, Equals, "test/test")
	c.Assert(s.client.Get(te), NotNil)

	c.Assert(s.client.Set(te), IsNil)

	te = newTestEntity("test", "")
	c.Assert(s.client.Get(te), IsNil)
	c.Assert(te.SomeData, Equals, "data")

	if Driver == "etcd" {
		client, err := getEtcdClient()
		c.Assert(err, IsNil)
		_, err = client.Get(context.Background(), "/volplugin/test/test", nil)
		c.Assert(err, IsNil)
	} else if Driver == "consul" {
		client, err := getConsulClient()
		c.Assert(err, IsNil)
		_, _, err = client.KV().Get("volplugin/test/test", nil)
		c.Assert(err, IsNil)
	} else {
		fmt.Println("invalid driver", Driver)
		c.Fail()
	}

	entities, err := s.client.List(te)
	c.Assert(err, IsNil)
	c.Assert(len(entities), Equals, 1)
	c.Assert(entities[0].(*testEntity).Name, Equals, "test")
	c.Assert(entities[0].(*testEntity).SomeData, Equals, "data")

	c.Assert(s.client.Delete(te), IsNil)
	c.Assert(s.client.Get(te), NotNil)
}

func (s *testSuite) TestWatch(c *C) {
	te := newTestEntity("test", "")

	retChan, errChan := s.client.Watch(te)

	// consul will compress writes before delivering watch messages, so we can't
	// rely on the routine below getting the right data for each iteration unless
	// we background this.
	go func() {
		for i := 0; i < 10; i++ {
			te2 := te.Copy().(*testEntity)
			key := fmt.Sprintf("data%d", i)
			te2.SomeData = key
			setKey(te2)
		}
	}()

	for i := 0; i < 10; i++ {
		select {
		case err := <-errChan:
			c.Assert(err, IsNil, Commentf("select: %v", err)) // this will always fail, assert is just to raise the error.
		case ent := <-retChan:
			te, ok := ent.(*testEntity)
			c.Assert(ok, Equals, true)
			c.Assert(te, NotNil)
			c.Assert(te.Name, Equals, "test")
			c.Assert(te.SomeData, Equals, fmt.Sprintf("data%d", i))
		}
	}

	c.Assert(s.client.WatchStop(te), IsNil)
	for i := 0; i < 10; i++ {
		te2 := te.Copy().(*testEntity)
		te2.SomeData = fmt.Sprintf("data%d", i)
		setKey(te2)
	}

	select {
	case err := <-errChan:
		c.Assert(err, IsNil, Commentf("select 2: %v", err)) // this will always fail, assert is just to raise the error.
	case <-retChan:
		c.Error("Watch still available after stop")
	default:
	}
}

func (s *testSuite) TestHooks(c *C) {
	triggered := &struct{ i int }{i: 0}

	te := newTestEntity("test", "")
	triggerFunc := func(c db.Client, te db.Entity) error {
		triggered.i++
		return nil
	}

	te.hooks.PreSet = triggerFunc
	c.Assert(s.client.Set(te), IsNil)
	c.Assert(triggered.i, Equals, 1)

	triggered.i = 0
	te.hooks.PostSet = triggerFunc
	c.Assert(s.client.Set(te), IsNil)
	c.Assert(triggered.i, Equals, 2)

	triggered.i = 0
	te.hooks.PreSet = nil
	te.hooks.PostSet = nil
	c.Assert(s.client.Set(te), IsNil)
	c.Assert(triggered.i, Equals, 0)

	te.hooks.PreGet = triggerFunc
	c.Assert(s.client.Get(te), IsNil)
	c.Assert(triggered.i, Equals, 1)

	triggered.i = 0
	te.hooks.PostGet = triggerFunc
	c.Assert(s.client.Get(te), IsNil)
	c.Assert(triggered.i, Equals, 2)

	triggered.i = 0
	te.hooks.PreGet = nil
	te.hooks.PostGet = nil
	c.Assert(s.client.Get(te), IsNil)
	c.Assert(triggered.i, Equals, 0)

	te.hooks.PreDelete = triggerFunc
	c.Assert(s.client.Delete(te), IsNil)
	c.Assert(triggered.i, Equals, 1)

	// to test these delete calls, we have to re-set the data.
	c.Assert(s.client.Set(te), IsNil)

	triggered.i = 0
	te.hooks.PostDelete = triggerFunc
	c.Assert(s.client.Delete(te), IsNil)
	c.Assert(triggered.i, Equals, 2)

	// to test these delete calls, we have to re-set the data.
	c.Assert(s.client.Set(te), IsNil)

	triggered.i = 0
	te.hooks.PreDelete = nil
	te.hooks.PostDelete = nil
	c.Assert(s.client.Delete(te), IsNil)
	c.Assert(triggered.i, Equals, 0)
}
