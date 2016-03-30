package systemtests

import (
	"fmt"
	"os"
	. "testing"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/vagrantssh"
)

type systemtestSuite struct {
	vagrant vagrantssh.Vagrant
}

var _ = Suite(&systemtestSuite{})

func TestSystem(t *T) {
	if os.Getenv("HOST_TEST") != "" {
		os.Exit(0)
	}

	TestingT(t)
}

func (s *systemtestSuite) SetUpTest(c *C) {
	c.Assert(s.rebootstrap(), IsNil)
}

func (s *systemtestSuite) SetUpSuite(c *C) {
	log.Infof("Bootstrapping system tests")
	s.vagrant = vagrantssh.Vagrant{}
	c.Assert(s.vagrant.Setup(false, "", 3), IsNil)

	for _, service := range []string{"volplugin", "volmaster", "volsupervisor"} {
		for _, node := range s.vagrant.GetNodes() {
			node.RunCommand(fmt.Sprintf("sudo systemctl stop %s", service))
			node.RunCommand(fmt.Sprintf("sudo systemctl disable %s", service))
		}
	}

	c.Assert(s.restartDocker(), IsNil)
	c.Assert(s.pullDebian(), IsNil)

	out, err := s.uploadIntent("policy1", "policy1")
	c.Assert(err, IsNil, Commentf("output: %s", out))
}
