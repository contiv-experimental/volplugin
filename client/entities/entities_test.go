package entities

import (
	"os/exec"
	. "testing"

	"golang.org/x/net/context"

	. "gopkg.in/check.v1"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/client"
	"github.com/contiv/volplugin/client/impl/etcd"
)

type entitySuite struct {
	client client.Client
}

var _ = Suite(&entitySuite{})

func TestEntities(t *T) { TestingT(t) }

func (s *entitySuite) SetUpTest(c *C) {
	errored.AlwaysDebug = true
	errored.AlwaysTrace = true
	exec.Command("/bin/sh", "-c", "etcdctl rm --recursive /volplugin /testing /test /watch").Run()
	var err error
	s.client, err = etcd.NewClient([]string{"http://127.0.0.1:2379"}, "volplugin")
	c.Assert(err, IsNil)
	client.Init(s.client)
}

func (s *entitySuite) setKey(path *client.Pather, value []byte) {
	if err := s.client.Set(context.Background(), path, value, nil); err != nil {
		panic(err)
	}
}
