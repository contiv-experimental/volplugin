package systemtests

import (
	"os"
	"strings"
	. "testing"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/vagrantssh"
)

type systemtestSuite struct {
	vagrant vagrantssh.Vagrant
	nodeMap map[string]vagrantssh.TestbedNode
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

func (s *systemtestSuite) TearDownTest(c *C) {
	if c.Failed() && os.Getenv("CONTIV_SOE") != "" {
		log.Infof("Test failed and SOE set. Terminating immediately")
		os.Exit(1)
	}
}

func (s *systemtestSuite) TearDownSuite(c *C) {
	if c.Failed() && os.Getenv("CONTIV_SOE") != "" {
		log.Infof("SOE set; aborting")
		os.Exit(1)
	}

	log.Infof("Tearing down system test facilities")

	s.clearContainers()
	s.clearVolumes()
	s.restartDocker()

	c.Assert(s.vagrant.IterateNodes(stopVolplugin), IsNil)
	c.Assert(stopVolmaster(s.vagrant.GetNode("mon0")), IsNil)
}

func (s *systemtestSuite) SetUpSuite(c *C) {
	log.Infof("Bootstrapping system tests")

	s.nodeMap = map[string]vagrantssh.TestbedNode{}
	s.vagrant = vagrantssh.Vagrant{}
	c.Assert(s.vagrant.Setup(false, "", 3), IsNil)
	for _, node := range s.vagrant.GetNodes() {
		s.nodeMap[node.GetName()] = node
	}

	err := s.clearContainers()
	if err != nil && !strings.Contains(err.Error(), "Process exited with: 123") {
		c.Fatal(err)
	}

	c.Assert(s.pullDebian(), IsNil)
	c.Assert(s.rebootstrap(), IsNil)

	_, err = s.uploadIntent("tenant1", "intent1")
	c.Assert(err, IsNil)
}
