package systemtests

import (
	"encoding/json"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/contiv/volplugin/config"
)

func (s *systemtestSuite) TestVolCLITenant(c *C) {
	intent1, err := s.readIntent("testdata/intent1.json")
	c.Assert(err, IsNil)

	intent2, err := s.readIntent("testdata/intent2.json")
	c.Assert(err, IsNil)

	_, err = s.volcli("tenant upload test1 < /testdata/intent1.json")
	c.Assert(err, IsNil)

	defer func() {
		_, err := s.volcli("tenant delete test1")
		c.Assert(err, IsNil)

		_, err = s.volcli("tenant get test1")
		c.Assert(err, NotNil)
	}()

	_, err = s.volcli("tenant upload test2 < /testdata/intent2.json")
	c.Assert(err, IsNil)

	defer func() {
		_, err := s.volcli("tenant delete test2")
		c.Assert(err, IsNil)

		_, err = s.volcli("tenant get test2")
		c.Assert(err, NotNil)
	}()

	out, err := s.volcli("tenant get test1")
	c.Assert(err, IsNil)

	intentTarget := &config.TenantConfig{}
	c.Assert(json.Unmarshal([]byte(out), intentTarget), IsNil)
	intent1.FileSystems = map[string]string{"ext4": "mkfs.ext4 -m0 %"}

	c.Assert(intent1, DeepEquals, intentTarget)
	c.Assert(err, IsNil)

	out, err = s.volcli("tenant get test2")
	c.Assert(err, IsNil)
	intentTarget = &config.TenantConfig{}

	c.Assert(json.Unmarshal([]byte(out), intentTarget), IsNil)
	intent2.FileSystems = map[string]string{"ext4": "mkfs.ext4 -m0 %"}
	c.Assert(intent2, DeepEquals, intentTarget)

	out, err = s.volcli("tenant list")
	c.Assert(err, IsNil)

	// matches assertion below doesn't handle newlines too well
	out = strings.Replace(out, "\n", " ", -1)

	c.Assert(out, Matches, ".*test1.*")
	c.Assert(out, Matches, ".*test2.*")
}

func (s *systemtestSuite) TestVolCLIVolume(c *C) {
	// XXX note that this is removed as a standard part of the tests and may error,
	// so we don't check it.
	defer s.volcli("volume remove tenant1/foo")

	c.Assert(s.createVolume("mon0", "tenant1", "foo", nil), IsNil)

	_, err := s.docker("run --rm -v tenant1/foo:/mnt debian ls")
	c.Assert(err, IsNil)

	out, err := s.volcli("volume list tenant1")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "foo")

	out, err = s.volcli("volume list-all")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "tenant1/foo")

	out, err = s.volcli("volume get tenant1/foo")
	c.Assert(err, IsNil)

	cfg := &config.VolumeConfig{}

	c.Assert(json.Unmarshal([]byte(out), cfg), IsNil)

	cfg.Options.FileSystem = "ext4"

	intent1, err := s.readIntent("testdata/intent1.json")
	c.Assert(err, IsNil)

	intent1.DefaultVolumeOptions.FileSystem = "ext4"

	c.Assert(intent1.DefaultVolumeOptions, DeepEquals, *cfg.Options)

	_, err = s.volcli("volume remove tenant1/foo")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume create tenant1/foo")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume remove tenant1/foo")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume get tenant1/foo")
	c.Assert(err, NotNil)

	_, err = s.volcli("volume create tenant1/foo --opt snapshots=false")
	c.Assert(err, IsNil)

	out, err = s.volcli("volume get tenant1/foo")
	c.Assert(err, IsNil)

	cfg = &config.VolumeConfig{}
	c.Assert(json.Unmarshal([]byte(out), cfg), IsNil)
	cfg.Options.FileSystem = "ext4"
	intent1, err = s.readIntent("testdata/intent1.json")
	c.Assert(err, IsNil)
	intent1.DefaultVolumeOptions.FileSystem = "ext4"
	intent1.DefaultVolumeOptions.UseSnapshots = false
	c.Assert(intent1.DefaultVolumeOptions, DeepEquals, *cfg.Options)
}

func (s *systemtestSuite) TestVolCLIUse(c *C) {
	c.Assert(s.createVolume("mon0", "tenant1", "foo", nil), IsNil)

	id, err := s.docker("run -itd -v tenant1/foo:/mnt debian sleep infinity")
	c.Assert(err, IsNil)

	out, err := s.volcli("use list")
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(out), Equals, "tenant1/foo")

	out, err = s.volcli("use get tenant1/foo")
	c.Assert(err, IsNil)

	ut := &config.UseConfig{}
	c.Assert(json.Unmarshal([]byte(out), ut), IsNil)
	c.Assert(ut.Volume, NotNil)
	c.Assert(ut.Hostname, Equals, "mon0")

	_, err = s.volcli("use force-remove tenant1/foo")
	c.Assert(err, IsNil)

	out, err = s.volcli("use list")
	c.Assert(err, IsNil)
	c.Assert(out, Equals, "")

	_, err = s.docker("rm -f " + id)
	c.Assert(err, IsNil)

	_, err = s.docker("volume rm tenant1/foo")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume remove tenant1/foo")
	c.Assert(err, IsNil)

	// the defer comes ahead of time here because of concerns that volume create
	// will half-create a volume
	defer s.purgeVolume("mon0", "tenant1", "foo", true)
	_, err = s.volcli("volume create tenant1/foo")
	c.Assert(err, IsNil)

	// ensure that double-create does nothing (for now, at least)
	_, err = s.volcli("volume create tenant1/foo")
	c.Assert(err, IsNil)

	_, err = s.volcli("volume get tenant1/foo")
	c.Assert(err, IsNil)

	// this test should never fail; we should always fail because of an exit code
	// instead, which would happen above.
	c.Assert(out, Equals, "")
}
