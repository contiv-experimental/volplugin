package test

import (
	"fmt"
	"os/exec"
	. "testing"

	"golang.org/x/net/context"

	. "gopkg.in/check.v1"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/impl/etcd"
	"github.com/contiv/volplugin/db/jsonio"
	"github.com/coreos/etcd/client"
)

var etcdHosts = []string{"http://127.0.0.1:2379"}

type testSuite struct {
	client db.Client
}

var _ = Suite(&testSuite{})

func TestDB(t *T) { TestingT(t) }

func (s *testSuite) SetUpTest(c *C) {
	errored.AlwaysDebug = true
	errored.AlwaysTrace = true
	exec.Command("/bin/sh", "-c", "etcdctl rm --recursive /volplugin /testing /test /watch").Run()
	var err error
	s.client, err = etcd.NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil)
}

func getEtcdClient() (client.KeysAPI, error) {
	ec, err := client.New(client.Config{Endpoints: etcdHosts})
	if err != nil {
		return nil, err
	}

	return client.NewKeysAPI(ec), nil
}

func setKey(value db.Entity) {
	client, err := getEtcdClient()
	if err != nil {
		panic(err)
	}

	path, err := value.Path()
	if err != nil {
		panic(err)
	}

	content, err := jsonio.Write(value)
	if err != nil {
		panic(err)
	}

	if _, err := client.Set(context.Background(), fmt.Sprintf("/volplugin/%v", path), string(content), nil); err != nil {
		panic(err)
	}
}
