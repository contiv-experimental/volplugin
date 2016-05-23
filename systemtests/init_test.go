package systemtests

import (
	"fmt"
	"os"
	"strings"
	. "testing"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/vagrantssh"
)

type systemtestSuite struct {
	vagrant vagrantssh.Vagrant
	mon0ip  string
}

var _ = Suite(&systemtestSuite{})

func TestSystem(t *T) {
	if os.Getenv("HOST_TEST") != "" {
		os.Exit(0)
	}

	if os.Getenv("DEBUG_TEST") != "" {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled")
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

	stopServices := []string{"volplugin", "volmaster", "volsupervisor"}
	startServices := []string{"ceph.target", "etcd"}

	/* FIXME too volatile. The intent was to ensure ceph was not a factor in some
	   of our work. I'm not sure this is as necessary now, but we should do it if
	   we have the time to do it properly. We'll need better orchestration here.

		if cephDriver() {
			startServices = append(startServices, "ceph.target")
		} else {
			stopServices = append(stopServices, "ceph.target")
		}

	*/

	nodelen := len(s.vagrant.GetNodes())
	sync := make(chan struct{}, nodelen)

	for _, service := range stopServices {
		for _, node := range s.vagrant.GetNodes() {
			log.Infof("Stopping %q service", service)
			go func(node vagrantssh.TestbedNode) {
				node.RunCommand(fmt.Sprintf("sudo systemctl stop %s", service))
				sync <- struct{}{}
			}(node)
		}
	}

	for i := 0; i < nodelen; i++ {
		<-sync
	}

	for _, service := range startServices {
		for _, node := range s.vagrant.GetNodes() {
			log.Infof("Starting %q service", service)
			go func(node vagrantssh.TestbedNode) {
				node.RunCommand(fmt.Sprintf("sudo systemctl start %s", service))
				sync <- struct{}{}
			}(node)
		}
	}

	for i := 0; i < nodelen; i++ {
		<-sync
	}

	if nfsDriver() {
		log.Info("NFS Driver detected: configuring exports.")
		c.Assert(s.createExports(), IsNil)
		ip, err := s.mon0cmd(`ip addr show dev enp0s8 | grep inet | head -1 | awk "{ print \$2 }" | awk -F/ "{ print \$1 }"`)
		log.Infof("mon0's ip is %s", strings.TrimSpace(ip))
		c.Assert(err, IsNil)
		s.mon0ip = strings.TrimSpace(ip)
	}

	c.Assert(s.clearContainers(), IsNil)
	c.Assert(s.restartDocker(), IsNil)
	c.Assert(s.pullDebian(), IsNil)

	out, err := s.uploadIntent("policy1", "policy1")
	c.Assert(err, IsNil, Commentf("output: %s", out))
}

func (s *systemtestSuite) TearDownSuite(c *C) {
	if cephDriver() && os.Getenv("NO_TEARDOWN") == "" {
		s.clearRBD()
	}
}
