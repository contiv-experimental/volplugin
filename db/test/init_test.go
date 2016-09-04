package test

import (
	"fmt"
	"os"
	"os/exec"
	. "testing"
	"time"

	"golang.org/x/net/context"

	. "gopkg.in/check.v1"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/impl/consul"
	"github.com/contiv/volplugin/db/impl/etcd"
	"github.com/contiv/volplugin/db/jsonio"
	"github.com/coreos/etcd/client"
	"github.com/hashicorp/consul/api"
)

var Driver = "etcd"

func init() {
	if os.Getenv("DRIVER") != "" {
		Driver = os.Getenv("DRIVER")
	}

	logrus.Infof("Enabling driver %q", Driver)
}

var etcdHosts = []string{"http://127.0.0.1:2379"}

type testSuite struct {
	client db.Client
}

var _ = Suite(&testSuite{})

func TestDB(t *T) { TestingT(t) }

func IsConsul() bool {
	return Driver == "consul"
}

func IsEtcd() bool {
	return Driver == "etcd"
}

func (s *testSuite) SetUpSuite(c *C) {
	if os.Getenv("DEBUG") != "" {
		errored.AlwaysDebug = true
		errored.AlwaysTrace = true
		logrus.SetLevel(logrus.DebugLevel)
	}
	var err error

	switch Driver {
	case "etcd":
		s.client, err = etcd.NewClient(etcdHosts, "volplugin")
		c.Assert(err, IsNil)
	case "consul":
		s.client, err = consul.NewClient("volplugin", &api.Config{Address: ":8500"})
		c.Assert(err, IsNil)
	}
}

func (s *testSuite) SetUpTest(c *C) {
	switch Driver {
	case "etcd":
		exec.Command("/bin/sh", "-c", "etcdctl rm --recursive /volplugin /testing /test /watch").Run()
	case "consul":
		startConsul(c)
		cli, err := getConsulClient()
		c.Assert(err, IsNil)
		_, err = cli.KV().DeleteTree("volplugin", nil)
		c.Assert(err, IsNil)
	}
}

func startConsul(c *C) {
	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl start consul").Run(), IsNil)
	for {
		if err := exec.Command("/bin/sh", "-c", "consul info | grep -q 'members = 3'").Run(); err == nil {
			break
		}

		time.Sleep(time.Second / 4)
	}
}
func getConsulClient() (*api.Client, error) {
	return api.NewClient(&api.Config{Address: ":8500"})
}

func getEtcdClient() (client.KeysAPI, error) {
	ec, err := client.New(client.Config{Endpoints: etcdHosts})
	if err != nil {
		return nil, err
	}

	return client.NewKeysAPI(ec), nil
}

func setKey(value db.Entity) {
	path, err := value.Path()
	if err != nil {
		panic(err)
	}

	content, err := jsonio.Write(value)
	if err != nil {
		panic(err)
	}

	switch Driver {
	case "etcd":
		client, err := getEtcdClient()
		if err != nil {
			panic(err)
		}

		if _, err := client.Set(context.Background(), fmt.Sprintf("/volplugin/%v", path), string(content), nil); err != nil {
			panic(err)
		}
	case "consul":
		client, err := getConsulClient()
		if err != nil {
			panic(err)
		}

		if _, err := client.KV().Put(&api.KVPair{Key: fmt.Sprintf("volplugin/%v", path), Value: content}, nil); err != nil {
			panic(err)
		}
	}
}
