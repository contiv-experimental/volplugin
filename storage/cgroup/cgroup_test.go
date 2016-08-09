package cgroup

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	. "testing"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"

	. "gopkg.in/check.v1"
)

type cgroupSuite struct{}

var _ = Suite(&cgroupSuite{})

func TestCGroup(t *T) { TestingT(t) }

func (s *cgroupSuite) TestApplyCGroupRateLimit(c *C) {
	err := ApplyCGroupRateLimit("/sys", config.RuntimeOptions{
		RateLimit: config.RateLimitConfig{
			WriteBPS: 123456,
			ReadBPS:  654321,
		},
	}, &storage.Mount{DevMajor: 253, DevMinor: 0})
	c.Assert(err, IsNil)

	defer func() {
		ApplyCGroupRateLimit("/sys", config.RuntimeOptions{
			RateLimit: config.RateLimitConfig{
				WriteBPS: 0,
				ReadBPS:  0,
			},
		}, &storage.Mount{DevMajor: 253, DevMinor: 0})
	}()

	content, err := ioutil.ReadFile(filepath.Join("/sys", writeBPSFile))
	c.Assert(err, IsNil)
	c.Assert(string(bytes.TrimSpace(content)), Matches, `^253:0 123456`)

	content, err = ioutil.ReadFile(filepath.Join("/sys", readBPSFile))
	c.Assert(err, IsNil)
	c.Assert(string(bytes.TrimSpace(content)), Matches, `^253:0 654321`)
}
