package systemtests

import (
	"os"
	"strings"
	. "testing"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
	utils "github.com/contiv/systemtests-utils"
)

var orderedNodes []utils.TestbedNode

type systemtestSuite struct {
	vagrant utils.Vagrant
	nodeMap map[string]utils.TestbedNode
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
	if os.Getenv("CONTIV_SOE") != "" {
		log.Infof("SOE set. Terminating immediately")
		os.Exit(1)
	}
}

func (s *systemtestSuite) TearDownSuite(c *C) {
	if os.Getenv("NO_TEARDOWN") != "" || os.Getenv("CONTIV_SOE") != "" {
		os.Exit(0)
	}

	log.Infof("Tearing down system test facilities")

	s.clearContainers()
	s.clearVolumes()
	s.restartDocker()

	c.Assert(s.vagrant.IterateNodes(stopVolplugin), IsNil)
	c.Assert(stopVolmaster(s.vagrant.GetNode("mon0")), IsNil)

	reversedNodes := []utils.TestbedNode{}

	for i := len(orderedNodes) - 1; i > -1; i-- {
		reversedNodes = append(reversedNodes, orderedNodes[i])
	}

	c.Assert(utils.StopEtcd(reversedNodes), IsNil)
}

func (s *systemtestSuite) SetUpSuite(c *C) {
	log.Infof("Bootstrapping system tests")

	s.nodeMap = map[string]utils.TestbedNode{}
	s.vagrant = utils.Vagrant{}
	c.Assert(s.vagrant.Setup(false, "", 3), IsNil)
	for _, node := range s.vagrant.GetNodes() {
		s.nodeMap[node.GetName()] = node
	}

	orderedNodes = []utils.TestbedNode{s.vagrant.GetNode("mon0"), s.vagrant.GetNode("mon1"), s.vagrant.GetNode("mon2")}

	c.Assert(s.restartDocker(), IsNil)
	err := s.clearContainers()
	if err != nil && !strings.Contains(err.Error(), "Process exited with: 123") {
		c.Fatal(err)
	}

	c.Assert(s.pullUbuntu(), IsNil)
	c.Assert(utils.StartEtcd(orderedNodes), IsNil)
	c.Assert(s.rebootstrap(), IsNil)

	_, err = s.uploadIntent("tenant1", "intent1")
	c.Assert(err, IsNil)
}
