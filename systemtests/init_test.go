package systemtests

import (
	"os"
	"strconv"
	"strings"
	. "testing"

	. "gopkg.in/check.v1"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/remotessh"
)

type systemtestSuite struct {
	vagrant remotessh.Vagrant
	mon0ip  string
}

var (
	batteryIterations int
	defaultIterations int64 = 5
)

var _ = Suite(&systemtestSuite{})

func TestSystem(t *T) {
	if os.Getenv("HOST_TEST") != "" {
		os.Exit(0)
	}

	if os.Getenv("DEBUG_TEST") != "" {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("Debug logging enabled")
	}

	TestingT(t)
}

func (s *systemtestSuite) SetUpTest(c *C) {
	c.Assert(s.rebootstrap(), IsNil)

	out, err := s.uploadIntent("policy1", "policy1")
	c.Assert(err, IsNil, Commentf("output: %s", out))
}

func (s *systemtestSuite) SetUpSuite(c *C) {
	logrus.Infof("Bootstrapping system tests")

	iter, err := strconv.ParseInt(os.Getenv("ITERATIONS"), 10, 64)
	if err != nil {
		iter = defaultIterations
	}

	batteryIterations = int(iter)

	s.vagrant = remotessh.Vagrant{}
	c.Assert(s.vagrant.Setup(false, []string{}, 3), IsNil)

	if nfsDriver() {
		logrus.Info("NFS Driver detected: configuring exports.")
		c.Assert(s.createExports(), IsNil)
		ip, err := s.mon0cmd(`ip addr show dev enp0s8 | grep inet | head -1 | awk "{ print \$2 }" | awk -F/ "{ print \$1 }"`)
		logrus.Infof("mon0's ip is %s", strings.TrimSpace(ip))
		c.Assert(err, IsNil)
		s.mon0ip = strings.TrimSpace(ip)
	}

	c.Assert(s.pullDebian(), IsNil)
}

func (s *systemtestSuite) TearDownSuite(c *C) {
	if cephDriver() && os.Getenv("NO_TEARDOWN") == "" {
		s.clearRBD()
	}
}
