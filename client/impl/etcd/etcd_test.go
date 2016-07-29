package etcd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	. "testing"

	"os/exec"

	. "gopkg.in/check.v1"

	"github.com/contiv/errored"
	"github.com/contiv/executor"
	"github.com/contiv/volplugin/client"
	"golang.org/x/net/context"

	etcdClient "github.com/coreos/etcd/client"
)

var etcdHosts = []string{"http://127.0.0.1:2379"}

type etcdSuite struct{}

var _ = Suite(&etcdSuite{})

func getEtcdClient() (etcdClient.KeysAPI, error) {
	ec, err := etcdClient.New(etcdClient.Config{Endpoints: etcdHosts})
	if err != nil {
		return nil, err
	}

	return etcdClient.NewKeysAPI(ec), nil
}

func setKey(path, value string) {
	client, err := getEtcdClient()
	if err != nil {
		panic(err)
	}
	if _, err := client.Set(context.Background(), path, value, nil); err != nil {
		panic(err)
	}
}

func TestEtcd(t *T) { TestingT(t) }

func (s *etcdSuite) SetUpSuite(c *C) {
	errored.AlwaysTrace = true
	errored.AlwaysDebug = true
}

func (s *etcdSuite) SetUpTest(c *C) {
	exec.Command("/bin/sh", "-c", "etcdctl rm --recursive /volplugin /testing /test /watch").Run()
}

func (s *etcdSuite) TestNewClient(c *C) {
	etcdClient, err := getEtcdClient()
	c.Assert(err, IsNil)
	_, err = etcdClient.Get(context.Background(), "/volplugin", nil)
	c.Assert(err, NotNil)

	volClient, err := NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil)
	c.Assert(volClient, NotNil)

	_, err = etcdClient.Get(context.Background(), "/volplugin", nil)
	c.Assert(err, IsNil)

	volClient, err = NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil, Commentf("%v", err))
	c.Assert(volClient, NotNil)

	volClient, err = NewClient(etcdHosts, "testing")
	c.Assert(err, IsNil)
	c.Assert(volClient, NotNil)

	_, err = etcdClient.Get(context.Background(), "/testing", nil)
	c.Assert(err, IsNil)
}

func (s *etcdSuite) TestCRUD(c *C) {
	volClient, err := NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil)

	etcdClient, err := getEtcdClient()
	c.Assert(err, IsNil)

	testPath, err := volClient.Path().Replace("test")
	c.Assert(err, IsNil)

	err = volClient.Set(context.Background(), testPath, nil, &client.SetOptions{Dir: true})
	c.Assert(err, IsNil)

	err = volClient.Set(context.Background(), testPath, nil, &client.SetOptions{Dir: true})
	c.Assert(err, NotNil)

	_, err = etcdClient.Get(context.Background(), "/volplugin/test", nil)
	c.Assert(err, IsNil)

	err = volClient.Delete(context.Background(), testPath, &client.DeleteOptions{Recursive: true})
	c.Assert(err, IsNil)

	_, err = etcdClient.Get(context.Background(), "/volplugin/test", nil)
	c.Assert(err, NotNil)

	err = volClient.Set(context.Background(), testPath, []byte("stuff"), nil)
	c.Assert(err, IsNil)

	err = volClient.Set(context.Background(), testPath, []byte("stuff2"), &client.SetOptions{Exist: client.PrevNoExist})
	c.Assert(err, NotNil)

	err = volClient.Set(context.Background(), testPath, []byte("stuff2"), &client.SetOptions{Value: []byte("stuff2")})
	c.Assert(err, NotNil)

	err = volClient.Set(context.Background(), testPath, []byte("stuff2"), &client.SetOptions{Exist: client.PrevExist})
	c.Assert(err, IsNil)

	result, err := volClient.Get(context.Background(), testPath, nil)
	c.Assert(err, IsNil)
	c.Assert(string(result.Value), Equals, "stuff2")

	err = volClient.Delete(context.Background(), testPath, &client.DeleteOptions{Value: []byte("Stuff")})
	c.Assert(err, NotNil)

	err = volClient.Delete(context.Background(), testPath, &client.DeleteOptions{Value: []byte("stuff2")})
	c.Assert(err, IsNil)

	_, err = volClient.Get(context.Background(), testPath, nil)
	c.Assert(err, NotNil)

	quuxPath, err := testPath.Append("quux")
	c.Assert(err, IsNil)

	err = volClient.Set(context.Background(), quuxPath, []byte("stuff2"), &client.SetOptions{Exist: client.PrevExist})
	c.Assert(err, NotNil)

	err = volClient.Set(context.Background(), quuxPath, []byte("stuff2"), nil)
	c.Assert(err, IsNil)

	err = volClient.Delete(context.Background(), quuxPath, &client.DeleteOptions{Recursive: true})
	c.Assert(err, IsNil)

	_, err = volClient.Get(context.Background(), quuxPath, nil)
	c.Assert(err, NotNil)
}

func (s *etcdSuite) TestDump(c *C) {
	volClient, err := NewClient(etcdHosts, "volplugin")
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

func (s *etcdSuite) TestWatch(c *C) {
	volClient, err := NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil)

	channel := make(chan interface{}, 10)

	feeder := func(node *client.Node, channel chan interface{}) error {
		channel <- node
		return nil
	}

	watchPath, err := volClient.Path().Replace("watch")
	c.Assert(err, IsNil)

	stopChan, errChan := volClient.Watch(watchPath, channel, true, feeder)
	defer func(stopChan chan struct{}) { stopChan <- struct{}{} }(stopChan)

	for i := 0; i < 10; i++ {
		setKey(fmt.Sprintf("/volplugin/watch/test%d", i), fmt.Sprintf("%d", i))
	}

	for i := 0; i < 10; i++ {
		select {
		case err := <-errChan:
			c.Assert(err, IsNil, Commentf("select: %v", err)) // this will always fail, assert is just to raise the error.
		case intf := <-channel:
			node, ok := intf.(*client.Node)
			c.Assert(ok, Equals, true)
			c.Assert(node, NotNil)
			c.Assert(node.Key, Equals, fmt.Sprintf("/volplugin/watch/test%d", i))
			c.Assert(string(node.Value), Equals, fmt.Sprintf("%d", i))
			c.Assert(node.Dir, Equals, false)
		}
	}

	volClient.WatchStop(watchPath)

	for i := 0; i < 10; i++ {
		setKey(fmt.Sprintf("/volplugin/watch/test%d", i), "")
	}

	var x = 0

	for i := 0; i < 10; i++ {
		select {
		case <-channel:
			x++
		default:
		}
	}

	c.Assert(x, Equals, 0)
}
