package config

import (
	"github.com/contiv/volplugin/watch"
	. "gopkg.in/check.v1"
)

const (
	testPolicyName = "foobar"
)

func revisionCount(s *configSuite, c *C) int {
	revisions, err := s.tlc.ListPolicyRevisions(testPolicyName)
	if err != nil {
		// this is a "does not exist" because the policy hasn't been uploaded yet
		return 0
	}

	return len(revisions)
}

func createTestPolicy(s *configSuite, c *C) {
	err := s.tlc.CreatePolicyRevision(testPolicyName, "{}")
	c.Assert(err, IsNil)
}

func (s *configSuite) TestCreatePolicyRevision(c *C) {
	c.Assert(revisionCount(s, c), Equals, 0)
	createTestPolicy(s, c)
	c.Assert(revisionCount(s, c), Equals, 1)
}

// the logic for testing Get and List only differs in that testing Get verifies
// that the policy text is what was uploaded, so the two tests are combined here.
func (s *configSuite) TestGetAndListPolicyRevisions(c *C) {
	policyText := `{"foo":"bar"}`

	c.Assert(revisionCount(s, c), Equals, 0)

	err := s.tlc.CreatePolicyRevision(testPolicyName, policyText)
	c.Assert(err, IsNil)

	revisions, err := s.tlc.ListPolicyRevisions(testPolicyName)
	c.Assert(err, IsNil)
	c.Assert(len(revisions), Equals, 1)

	resp, err := s.tlc.GetPolicyRevision(testPolicyName, revisions[0])
	c.Assert(err, IsNil)
	c.Assert(resp, Equals, policyText)
}

func (s *configSuite) TestWatchForPolicyChanges(c *C) {
	activity := make(chan *watch.Watch)
	s.tlc.WatchForPolicyChanges(activity)

	createTestPolicy(s, c)

	w := <-activity

	watch.Stop(s.tlc.prefixed(rootPolicyArchive))

	changeset := w.Config.([]string)
	c.Assert(len(changeset), Equals, 2)

	name := changeset[0]
	revision := changeset[1]

	_, err := s.tlc.GetPolicyRevision(name, revision)
	c.Assert(err, IsNil)
}
