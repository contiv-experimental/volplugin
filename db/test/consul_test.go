package test

import (
	"os/exec"
	"time"

	. "gopkg.in/check.v1"

	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/impl/consul"
	"github.com/hashicorp/consul/api"
)

func (s *testSuite) TestNewConsulClient(c *C) {
	if !IsConsul() {
		c.Skip("not consul")
	}

	client, err := getConsulClient()
	c.Assert(err, IsNil)

	volClient, err := consul.NewClient("volplugin", &api.Config{})
	c.Assert(err, IsNil)
	c.Assert(volClient, NotNil)

	_, _, err = client.KV().Get("volplugin", nil)
	c.Assert(err, IsNil)

	_, err = consul.NewClient("/volplugin", &api.Config{})
	c.Assert(err, NotNil, Commentf("Consul keyspaces can't start with /"))
}

func (s *testSuite) TestConsulDown(c *C) {
	if !IsConsul() {
		c.Skip("not consul")
	}

	volClient, err := consul.NewClient("volplugin", &api.Config{})
	c.Assert(err, IsNil)

	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl stop consul").Run(), IsNil)
	for {
		if err := exec.Command("/bin/sh", "-c", "consul info | grep -q 'members = 2'").Run(); err != nil {
			break
		}

		time.Sleep(time.Second / 4)
	}

	defer startConsul(c)
	policy := db.NewPolicy("policy")
	c.Assert(volClient.Set(policy), NotNil)
	c.Assert(policy, DeepEquals, db.NewPolicy("policy"))
}
