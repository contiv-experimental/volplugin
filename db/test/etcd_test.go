package test

import (
	"os/exec"
	"time"

	. "gopkg.in/check.v1"

	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/impl/etcd"
	"golang.org/x/net/context"
)

func (s *testSuite) TestNewEtcdClient(c *C) {
	if !IsEtcd() {
		c.Skip("not etcd")
	}

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

func (s *testSuite) TestEtcdDown(c *C) {
	if !IsEtcd() {
		c.Skip("not etcd")
	}

	volClient, err := etcd.NewClient(etcdHosts, "volplugin")
	c.Assert(err, IsNil)

	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl stop etcd").Run(), IsNil)
	for {
		if err := exec.Command("etcdctl", "cluster-health").Run(); err != nil {
			break
		}

		time.Sleep(time.Second / 4)
	}

	defer func() {
		c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl start etcd").Run(), IsNil)
		for {
			if err := exec.Command("etcdctl", "cluster-health").Run(); err == nil {
				break
			}

			time.Sleep(time.Second / 4)
		}
	}()

	policy := db.NewPolicy("policy")
	c.Assert(volClient.Set(policy), NotNil)
	c.Assert(policy, DeepEquals, db.NewPolicy("policy"))
}
