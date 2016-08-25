package config

import (
	"os/exec"
	"time"

	. "gopkg.in/check.v1"
)

func stopStartEtcd(c *C, f func()) {
	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl stop etcd").Run(), IsNil)
	for {
		if err := exec.Command("/bin/sh", "-c", "etcdctl cluster-health").Run(); err != nil {
			break
		}

		time.Sleep(time.Second / 4)
	}

	f()

	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl start etcd").Run(), IsNil)
	for {
		if err := exec.Command("/bin/sh", "-c", "etcdctl cluster-health").Run(); err == nil {
			break
		}
		time.Sleep(time.Second / 4)
	}
}
