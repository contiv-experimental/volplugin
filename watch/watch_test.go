package watch

import (
	"fmt"
	"os/exec"

	"golang.org/x/net/context"

	. "testing"

	. "gopkg.in/check.v1"

	"github.com/coreos/etcd/client"
)

type watchSuite struct{}

var _ = Suite(&watchSuite{})

func TestWatch(t *T) { TestingT(t) }

func setKey(path, value string) {
	etcdClient.Set(context.Background(), path, value, nil)
}

func (s *watchSuite) SetUpTest(c *C) {
	exec.Command("/bin/sh", "-c", "etcdctl rm --recursive /watch").Run()

	etcdCfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}

	etcdClient, err := client.New(etcdCfg)
	c.Assert(err, IsNil)

	Init(client.NewKeysAPI(etcdClient))
}

func (s *watchSuite) TestBasic(c *C) {
	w := NewWatcher(make(chan *Watch), "/watch", func(resp *client.Response, w *Watcher) {
		w.Channel <- &Watch{Key: resp.Node.Key, Config: true}
	})

	Create(w)

	val, ok := watchers["/watch"]
	c.Assert(ok, Equals, true)
	c.Assert(val, NotNil)

	for i := 0; i < 10; i++ {
		setKey(fmt.Sprintf("/watch/test%d", i), "")
	}

	select {
	case err := <-w.ErrorChannel:
		c.Assert(err, IsNil) // won't ever pass, but at least we'll get the message this way
	default:
	}

	for i := 0; i < 10; i++ {
		c.Assert((<-w.Channel).Config.(bool), Equals, true)
	}

	Stop(w.Path)

	for i := 0; i < 10; i++ {
		setKey(fmt.Sprintf("/watch/test%d", i), "")
	}

	var x = 0

	for i := 0; i < 10; i++ {
		select {
		case <-w.Channel:
			x++
		default:
		}
	}

	c.Assert(x, Equals, 0)
}
