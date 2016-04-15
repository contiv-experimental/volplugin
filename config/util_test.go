package config

import (
	"os/exec"
	"time"

	. "gopkg.in/check.v1"
)

func stopStartEtcd(c *C, f func()) {
	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl stop etcd").Run(), IsNil)
	time.Sleep(1 * time.Second)
	f()
	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl start etcd").Run(), IsNil)
	time.Sleep(15 * time.Second)
}
