package storage

import (
	. "gopkg.in/check.v1"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
)

func (s *storageSuite) TestSplitName(c *C) {
	failures := []string{
		"foo",
		"foo/",
		"/foo/bar",
		"foo/bar/",
		"foo/bar/quux",
	}

	successes := map[string][]string{
		"foo/bar":                      []string{"foo", "bar"},
		"policy-with-dashes/quux":      []string{"policy-with-dashes", "quux"},
		"policy_with_underscores/quux": []string{"policy_with_underscores", "quux"},
		"policy.with.periods/quux":     []string{"policy.with.periods", "quux"},
	}

	for _, fail := range failures {
		_, _, err := SplitName(fail)
		c.Assert(err, NotNil)
		c.Assert(err.(*errored.Error).Contains(errors.InvalidVolume), Equals, true)
	}

	for vol, results := range successes {
		policy, volume, err := SplitName(vol)
		c.Assert(err, IsNil)
		c.Assert(policy, Equals, results[0])
		c.Assert(volume, Equals, results[1])
	}
}
