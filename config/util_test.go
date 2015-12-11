package config

import (
	"os/exec"
	"time"

	. "gopkg.in/check.v1"
)

func stopStartEtcd(c *C, f func()) {
	defer time.Sleep(1 * time.Second)
	defer func() {
		// I have ABSOLUTELY no idea why CombinedOutput() is required here, but it is.
		_, err := exec.Command("/bin/sh", "-c", "sudo systemctl restart etcd").CombinedOutput()
		c.Assert(err, IsNil)
	}()

	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl start etcd").Run(), IsNil)
	c.Assert(exec.Command("/bin/sh", "-c", "sudo systemctl stop etcd").Run(), IsNil)
	time.Sleep(1 * time.Second)
	f()
}
