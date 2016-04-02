package systemtests

import (
	"encoding/json"
	"fmt"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage/backend/ceph"
)

func (s *systemtestSuite) TestVolCLIPolicy(c *C) {
	policy1, err := s.readIntent("testdata/policy1.json")
	c.Assert(err, IsNil)

	policy2, err := s.readIntent("testdata/policy2.json")
	c.Assert(err, IsNil)

	_, err = s.volcli("policy upload test1 < /testdata/policy1.json")
	c.Assert(err, IsNil)

	defer func() {
		_, err := s.volcli("policy delete test1")
		c.Assert(err, IsNil)

		_, err = s.volcli("policy get test1")
		c.Assert(err, NotNil)
	}()

	_, err = s.volcli("policy upload test2 < /testdata/policy2.json")
	c.Assert(err, IsNil)

	defer func() {
		_, err := s.volcli("policy delete test2")
		c.Assert(err, IsNil)

		_, err = s.volcli("policy get test2")
		c.Assert(err, NotNil)
	}()

	out, err := s.volcli("policy get test1")
	c.Assert(err, IsNil)

	intentTarget := config.NewPolicy(ceph.BackendName)
	c.Assert(json.Unmarshal([]byte(out), intentTarget), IsNil)
	policy1.FileSystems = map[string]string{"ext4": "mkfs.ext4 -m0 %"}

	c.Assert(policy1, DeepEquals, intentTarget)
	c.Assert(err, IsNil)

	out, err = s.volcli("policy get test2")
	c.Assert(err, IsNil)

	intentTarget = config.NewPolicy(ceph.BackendName)
	c.Assert(json.Unmarshal([]byte(out), intentTarget), IsNil)
	policy2.FileSystems = map[string]string{"ext4": "mkfs.ext4 -m0 %"}
	c.Assert(policy2, DeepEquals, intentTarget)

	out, err = s.volcli("policy list")
	c.Assert(err, IsNil)

	// matches assertion below doesn't handle newlines too well
	out = strings.Replace(out, "\n", " ", -1)

	c.Assert(out, Matches, ".*test1.*")
	c.Assert(out, Matches, ".*test2.*")
}

func (s *systemtestSuite) TestVolCLIPolicyNullDriver(c *C) {
	nullDriverIntent, err := s.readIntent("testdata/nulldriver.json")
	c.Assert(err, IsNil)
	_, err = s.volcli("policy upload test < /testdata/nulldriver.json")
	c.Assert(err, IsNil)

	defer func() {
		_, err := s.volcli("policy delete test")
		c.Assert(err, IsNil)

		_, err = s.volcli("policy get test")
		c.Assert(err, NotNil)
	}()

	out, err := s.volcli("policy get test")
	c.Assert(err, IsNil)
	intentTarget := config.NewPolicy("")
	c.Assert(json.Unmarshal([]byte(out), intentTarget), IsNil)
	nullDriverIntent.FileSystems = map[string]string{"ext4": "mkfs.ext4 -m0 %"}
	c.Assert(nullDriverIntent, DeepEquals, intentTarget)
}

func (s *systemtestSuite) TestVolCLIVolume(c *C) {
	// XXX note that this is removed as a standard part of the tests and may error,
	// so we don't check it.
	defer s.volcli("volume remove policy1/foo")

	c.Assert(s.createVolume("mon0", "policy1", "foo", nil), IsNil)

	_, err := s.docker("run --rm -v policy1/foo:/mnt alpine ls")
	c.Assert(err, IsNil)

	out, err := s.volcli("volume list policy1")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "foo")

	out, err = s.volcli("volume list-all")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "policy1/foo")

	out, err = s.volcli("volume get policy1/foo")
	c.Assert(err, IsNil)

	cfg := &config.Volume{}

	c.Assert(json.Unmarshal([]byte(out), cfg), IsNil)

	cfg.CreateOptions.FileSystem = "ext4"

	policy1, err := s.readIntent("testdata/policy1.json")
	c.Assert(err, IsNil)

	policy1.CreateOptions.FileSystem = "ext4"

	c.Assert(policy1.CreateOptions, DeepEquals, cfg.CreateOptions)

	_, err = s.volcli("volume remove policy1/foo")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume create policy1/foo")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume remove policy1/foo")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume get policy1/foo")
	c.Assert(err, NotNil)

	_, err = s.volcli("volume create policy1/foo --opt snapshots=false")
	c.Assert(err, IsNil)

	out, err = s.volcli("volume get policy1/foo")
	c.Assert(err, IsNil)

	cfg = &config.Volume{}
	c.Assert(json.Unmarshal([]byte(out), cfg), IsNil)
	cfg.CreateOptions.FileSystem = "ext4"
	policy1, err = s.readIntent("testdata/policy1.json")
	c.Assert(err, IsNil)
	policy1.CreateOptions.FileSystem = "ext4"
	policy1.RuntimeOptions.UseSnapshots = false
	c.Assert(policy1.CreateOptions, DeepEquals, cfg.CreateOptions)
	c.Assert(policy1.RuntimeOptions, DeepEquals, cfg.RuntimeOptions)
}

func (s *systemtestSuite) TestVolCLIVolumePolicyUpdate(c *C) {
	out, err := s.volcli("policy upload test1 < /testdata/policy1.json")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume create test1/foo")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("policy upload test1 < /testdata/policy2.json")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume create test1/bar")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	out, err = s.volcli("volume list-all")
	c.Assert(err, IsNil, Commentf("output: %s", out))

	// matches assertion below doesn't handle newlines too well
	out = strings.Replace(out, "\n", " ", -1)

	c.Assert(out, Matches, ".*test1/foo.*", Commentf("output: %s", out))
	c.Assert(out, Matches, ".*test1/bar.*", Commentf("output: %s", out))
}

func (s *systemtestSuite) TestVolCLIUse(c *C) {
	c.Assert(s.createVolume("mon0", "policy1", "foo", nil), IsNil)

	id, err := s.docker("run -itd -v policy1/foo:/mnt alpine sleep 10m")
	c.Assert(err, IsNil)

	out, err := s.volcli("use list")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "policy1/foo")

	out, err = s.volcli("use get policy1/foo")
	c.Assert(err, IsNil)

	ut := &config.UseMount{}
	c.Assert(json.Unmarshal([]byte(out), ut), IsNil)
	c.Assert(ut.Volume, NotNil)
	c.Assert(ut.Hostname, Equals, "mon0")

	out, err = s.volcli("use force-remove policy1/foo")
	fmt.Println(out)
	c.Assert(err, IsNil)

	out, err = s.volcli("use list")
	c.Assert(err, IsNil)
	c.Assert(out, Equals, "")

	_, err = s.docker("rm -f " + id)
	c.Assert(err, IsNil)

	_, err = s.docker("volume rm policy1/foo")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume remove policy1/foo")
	c.Assert(err, IsNil)

	// the defer comes ahead of time here because of concerns that volume create
	// will half-create a volume
	defer s.purgeVolume("mon0", "policy1", "foo", true)
	_, err = s.volcli("volume create policy1/foo")
	c.Assert(err, IsNil)

	// ensure that double-create does nothing (for now, at least)
	_, err = s.volcli("volume create policy1/foo")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume get policy1/foo")
	c.Assert(err, IsNil)

	// this test should never fail; we should always fail because of an exit code
	// instead, which would happen above.
	c.Assert(out, Equals, "")
}
