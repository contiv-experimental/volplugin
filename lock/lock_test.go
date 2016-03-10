package lock

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"

	. "testing"

	. "gopkg.in/check.v1"

	"github.com/contiv/volplugin/config"
)

type lockSuite struct {
	tlc *config.TopLevelConfig
}

var _ = Suite(&lockSuite{})

func TestLock(t *T) { TestingT(t) }

func (s *lockSuite) SetUpTest(c *C) {
	exec.Command("/bin/sh", "-c", "etcdctl rm --recursive /volplugin").Run()
	tlc, err := config.NewTopLevelConfig("/volplugin", []string{"http://127.0.0.1:2379"})
	if err != nil {
		c.Fatal(err)
	}

	s.tlc = tlc

	content, err := ioutil.ReadFile("policy.json")
	c.Assert(err, IsNil)

	policy := &config.PolicyConfig{}

	c.Assert(json.Unmarshal(content, policy), IsNil)

	s.tlc.PublishPolicy("policy", policy)
}

func (s *lockSuite) TestExecuteWithUseLock(c *C) {
	vc, err := s.tlc.CreateVolume(config.RequestCreate{Policy: "policy", Volume: "foo"})
	c.Assert(err, IsNil)
	uc := &config.UseMount{
		Volume:   vc,
		Reason:   ReasonCreate,
		Hostname: "mon0",
	}

	ch1 := make(chan bool, 1)

	driver := NewDriver(s.tlc)

	c.Assert(driver.ExecuteWithUseLock(uc, func(ld *Driver, uc config.UseLocker) error {
		ch1 <- true
		return nil
	}), IsNil)

	c.Assert(<-ch1, Equals, true)

	ch2 := make(chan int)

	// this channel is used to synchronize the below goroutine's knowledge that
	// the lock has been acquired. Otherwise, the tests will randomly hang, and
	// should.
	sync := make(chan struct{})

	go driver.ExecuteWithUseLock(uc, func(ld *Driver, uc config.UseLocker) error {
		sync <- struct{}{}
		ch2 <- 1
		return nil
	})

	<-sync

	chErr := make(chan error, 50)

	for i := 0; i < 50; i++ {
		uc := &config.UseMount{
			Volume:   vc,
			Reason:   ReasonCreate,
			Hostname: fmt.Sprintf("mon%d", i), // doubly ensure we try to write a use lock at this point
		}

		go func() {
			chErr <- driver.ExecuteWithUseLock(uc, func(ld *Driver, uc config.UseLocker) error {
				ch2 <- 2
				return nil
			})
		}()
	}

	for i := 0; i < 50; i++ {
		c.Assert(<-chErr, Equals, ErrPublish)
	}

	c.Assert(<-ch2, Equals, 1)
}
