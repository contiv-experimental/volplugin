package systemtests

import (
	"encoding/json"
	"fmt"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/lock"
)

func (s *systemtestSuite) TestVolCLIEmptyGlobal(c *C) {
	c.Assert(s.uploadGlobal("global-empty"), IsNil)

	out, err := s.volcli("global get")
	c.Assert(err, IsNil)

	target := &config.Global{}

	c.Assert(json.Unmarshal([]byte(out), target), IsNil, Commentf(out))
	c.Assert(config.NewGlobalConfig(), DeepEquals, target, Commentf("%q %#v", out, target))
}

func (s *systemtestSuite) TestVolCLIPolicy(c *C) {
	policy1, err := s.readIntent(fmt.Sprintf("testdata/%s/policy1.json", getDriver()))
	c.Assert(err, IsNil)

	policy2, err := s.readIntent(fmt.Sprintf("testdata/%s/policy2.json", getDriver()))
	c.Assert(err, IsNil)

	_, err = s.uploadIntent("test1", "policy1")
	c.Assert(err, IsNil)

	defer func() {
		_, err := s.volcli("policy delete test1")
		c.Assert(err, IsNil)

		_, err = s.volcli("policy get test1")
		c.Assert(err, NotNil)
	}()

	_, err = s.uploadIntent("test2", "policy2")
	c.Assert(err, IsNil)

	defer func() {
		_, err := s.volcli("policy delete test2")
		c.Assert(err, IsNil)

		_, err = s.volcli("policy get test2")
		c.Assert(err, NotNil)
	}()

	out, err := s.volcli("policy get test1")
	c.Assert(err, IsNil)

	intentTarget := config.NewPolicy()
	c.Assert(json.Unmarshal([]byte(out), intentTarget), IsNil)
	policy1.FileSystems = map[string]string{"ext4": "mkfs.ext4 -m0 %"}

	policy1.Name = "test1"
	c.Assert(policy1, DeepEquals, intentTarget)
	c.Assert(err, IsNil)

	out, err = s.volcli("policy get test2")
	c.Assert(err, IsNil)

	intentTarget = config.NewPolicy()
	c.Assert(json.Unmarshal([]byte(out), intentTarget), IsNil)
	policy2.FileSystems = map[string]string{"ext4": "mkfs.ext4 -m0 %"}
	policy2.Name = "test2"
	c.Assert(policy2, DeepEquals, intentTarget)

	out, err = s.volcli("policy list")
	c.Assert(err, IsNil)

	// matches assertion below doesn't handle newlines too well
	out = strings.Replace(out, "\n", " ", -1)

	c.Assert(out, Matches, ".*test1.*")
	c.Assert(out, Matches, ".*test2.*")
}

func (s *systemtestSuite) TestVolCLIPolicyNullDriver(c *C) {
	testDriverIntent, err := s.readIntent(fmt.Sprintf("testdata/%s/testdriver.json", getDriver()))
	c.Assert(err, IsNil)
	out, err := s.uploadIntent("test", "testdriver")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	defer func() {
		out, err := s.volcli("policy delete test")
		c.Assert(err, IsNil, Commentf("output: %s", out))

		out, err = s.volcli("policy get test")
		c.Assert(err, NotNil, Commentf("output: %s", out))
	}()

	out, err = s.volcli("policy get test")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	intentTarget := config.NewPolicy()
	c.Assert(json.Unmarshal([]byte(out), intentTarget), IsNil)
	testDriverIntent.Name = "test"
	testDriverIntent.FileSystems = map[string]string{"ext4": "mkfs.ext4 -m0 %"}
	c.Assert(testDriverIntent, DeepEquals, intentTarget)
}

func (s *systemtestSuite) TestVolCLIVolume(c *C) {
	out, err := s.volcli("volume list policy1")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	c.Assert(strings.TrimSpace(out), Equals, "")

	out, err = s.volcli("volume list-all")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	c.Assert(strings.TrimSpace(out), Equals, "")

	shortVolName := genRandomVolume()
	volName := fqVolume("policy1", shortVolName)
	// XXX note that this is removed as a standard part of the tests and may error,
	// so we don't check it.
	defer s.volcli("volume remove " + volName)

	c.Assert(s.createVolume("mon0", volName, nil), IsNil)

	out, err = s.dockerRun("mon0", false, false, volName, "ls")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume list policy1")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	c.Assert(strings.TrimSpace(out), Equals, shortVolName)

	out, err = s.volcli("volume list-all")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	c.Assert(strings.TrimSpace(out), Equals, volName)

	out, err = s.volcli("volume get " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	cfg := &config.Volume{}

	c.Assert(json.Unmarshal([]byte(out), cfg), IsNil)

	cfg.CreateOptions.FileSystem = "ext4"

	policy1, err := s.readIntent(fmt.Sprintf("testdata/%s/policy1.json", getDriver()))
	c.Assert(err, IsNil)

	policy1.Name = "policy1"
	policy1.CreateOptions.FileSystem = "ext4"

	c.Assert(policy1.CreateOptions, DeepEquals, cfg.CreateOptions)
	options := ""

	if nfsDriver() {
		s.mon0cmd("sudo mkdir -p -m 4777 /volplugin/policy1/foo")
		options = fmt.Sprintf("--opt mount=%s:/volplugin/policy1/foo", s.mon0ip)
	}

	out, err = s.volcli("volume remove " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli(fmt.Sprintf("volume create %s %s", volName, options))
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume remove " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume get " + volName)
	c.Assert(err, NotNil, Commentf("output: %s", out))

	out, err = s.volcli(fmt.Sprintf("volume create %s --opt snapshots=false %s", volName, options))
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume get " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	cfg = &config.Volume{}
	c.Assert(json.Unmarshal([]byte(out), cfg), IsNil)
	cfg.CreateOptions.FileSystem = "ext4"
	policy1, err = s.readIntent(fmt.Sprintf("testdata/%s/policy1.json", getDriver()))
	c.Assert(err, IsNil)
	policy1.CreateOptions.FileSystem = "ext4"
	policy1.RuntimeOptions.UseSnapshots = false
	c.Assert(policy1.CreateOptions, DeepEquals, cfg.CreateOptions)
	c.Assert(policy1.RuntimeOptions, DeepEquals, cfg.RuntimeOptions)
}

func (s *systemtestSuite) TestVolCLIVolumePolicyUpdate(c *C) {
	out, err := s.uploadIntent("test1", "policy1")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	volName := fqVolume("test1", genRandomVolume())

	options := ""

	if nfsDriver() {
		options = fmt.Sprintf("--opt mount=%s:/volplugin/%s", s.mon0ip, volName)
	}

	out, err = s.volcli(fmt.Sprintf("volume create %s %s", volName, options))
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.uploadIntent("test1", "policy2")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	volName2 := fqVolume("test1", genRandomVolume())

	out, err = s.volcli(fmt.Sprintf("volume create %s %s", volName2, options))
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume list-all")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	// matches assertion below doesn't handle newlines too well
	out = strings.Replace(out, "\n", " ", -1)

	c.Assert(out, Matches, fmt.Sprintf(".*%s.*", volName), Commentf("output: %s", out))
	c.Assert(out, Matches, fmt.Sprintf(".*%s.*", volName2), Commentf("output: %s", out))
}

func (s *systemtestSuite) TestVolCLIVolumeTakeSnapshot(c *C) {
	if !cephDriver() {
		c.Skip("Cannot test snapshots without ceph driver")
		return
	}

	out, err := s.uploadIntent("test1", "policy1")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	volName := fqVolume("test1", genRandomVolume())

	out, err = s.volcli("volume create " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume snapshot take " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))
}

func (s *systemtestSuite) TestVolCLIUse(c *C) {
	volName := fqVolume("policy1", genRandomVolume())

	c.Assert(s.createVolume("mon0", volName, nil), IsNil)

	id, err := s.dockerRun("mon0", false, true, volName, "sleep 10m")
	c.Assert(err, IsNil, Commentf("output: %s", id))

	out, err := s.volcli("use list")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	c.Assert(strings.TrimSpace(out), Equals, volName)

	out, err = s.volcli("use get " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	ut := &config.UseMount{}
	c.Assert(json.Unmarshal([]byte(out), ut), IsNil)
	c.Assert(ut.Volume, NotNil)
	c.Assert(ut.Hostname, Equals, "mon0")
	c.Assert(ut.Reason, Equals, lock.ReasonMount)

	out, err = s.mon0cmd("docker rm -f " + id)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("use list")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	c.Assert(out, Equals, "")

	out, err = s.mon0cmd("docker volume rm " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume remove " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	if cephDriver() {
		// the defer comes ahead of time here because of concerns that volume create
		// will half-create a volume
		defer s.purgeVolume("mon0", volName)
		out, err = s.volcli("volume create " + volName)
		c.Assert(err, IsNil, Commentf("output: %s", out))

		// ensure that double-create errors
		out, err = s.volcli("volume create " + volName)
		c.Assert(err, NotNil, Commentf("output: %s", out))

		out, err = s.volcli("volume get " + volName)
		c.Assert(err, IsNil, Commentf("output: %s", out))
	}
}

func (s *systemtestSuite) TestVolCLIRemoveForce(c *C) {
	volName := fqVolume("policy1", genRandomVolume())

	c.Assert(s.createVolume("mon0", volName, nil), IsNil)

	id, err := s.dockerRun("mon0", false, true, volName, "sleep 10m")
	c.Assert(err, IsNil, Commentf("output: %s", id))

	out, err := s.volcli("use list")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	c.Assert(strings.TrimSpace(out), Equals, volName)

	out, err = s.volcli("use get " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	ut := &config.UseMount{}
	c.Assert(json.Unmarshal([]byte(out), ut), IsNil)
	c.Assert(ut.Volume, NotNil)
	c.Assert(ut.Hostname, Equals, "mon0")
	c.Assert(ut.Reason, Equals, lock.ReasonMount)

	out, err = s.mon0cmd("docker rm -f " + id)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("remove -f " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("use list")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	c.Assert(out, Equals, "")

	out, err = s.mon0cmd("docker volume rm " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume remove " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))
}

func (s *systemtestSuite) TestVolCLIRemoveTimeout(c *C) {
	volName := fqVolume("policy1", genRandomVolume())

	c.Assert(s.createVolume("mon0", volName, nil), IsNil)

	id, err := s.dockerRun("mon0", false, true, volName, "sleep 10m")
	c.Assert(err, IsNil, Commentf("output: %s", id))

	out, err := s.volcli("use list")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	c.Assert(strings.TrimSpace(out), Equals, volName)

	out, err = s.volcli("use get " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	ut := &config.UseMount{}
	c.Assert(json.Unmarshal([]byte(out), ut), IsNil)
	c.Assert(ut.Volume, NotNil)
	c.Assert(ut.Hostname, Equals, "mon0")
	c.Assert(ut.Reason, Equals, lock.ReasonMount)

	out, err = s.mon0cmd("docker rm -f " + id)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("remove -t 2s " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("use list")
	c.Assert(err, IsNil, Commentf("output: %s", out))
	c.Assert(out, Equals, "")

	out, err = s.mon0cmd("docker volume rm " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume remove " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))
}

func (s *systemtestSuite) TestVolCLIRuntime(c *C) {
	if !cephDriver() {
		c.Skip("Only the ceph driver supports runtime parameters")
		return
	}

	volName := fqVolume("policy1", genRandomVolume())

	c.Assert(s.createVolume("mon0", volName, nil), IsNil)
	volcliOut, err := s.volcli("volume runtime get " + volName)
	c.Assert(err, IsNil)
	runtimeOptions := config.RuntimeOptions{}
	c.Assert(json.Unmarshal([]byte(volcliOut), &runtimeOptions), IsNil)

	volcliOut, err = s.volcli("volume get " + volName)
	c.Assert(err, IsNil)
	volume := &config.Volume{}
	c.Assert(json.Unmarshal([]byte(volcliOut), volume), IsNil)

	c.Assert(volume.RuntimeOptions, DeepEquals, runtimeOptions)
	c.Assert(volume.RuntimeOptions.Snapshot.Keep, Equals, uint(20))

	out, err := s.volcli(fmt.Sprintf("volume runtime upload %s < /testdata/runtime1.json", volName))
	c.Assert(err, IsNil, Commentf("output: %s", out))
	volcliOut, err = s.volcli("volume get " + volName)
	c.Assert(err, IsNil, Commentf("output: %s", out))
	volume = &config.Volume{}
	c.Assert(json.Unmarshal([]byte(volcliOut), volume), IsNil)
	c.Assert(volume.RuntimeOptions.Snapshot.Keep, Equals, uint(15))
}
