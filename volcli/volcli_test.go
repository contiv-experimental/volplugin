package volcli

import (
	"flag"
	. "testing"

	"github.com/codegangsta/cli"

	. "gopkg.in/check.v1"
)

type volcliSuite struct {
}

var _ = Suite(&volcliSuite{})

func TestVolcli(t *T) { TestingT(t) }

func (s *volcliSuite) TestErrorReturns(c *C) {
	testMap := map[string]struct {
		f    func(ctx *cli.Context) (bool, error)
		args []string
		err  error
	}{
		"policyUpload": {
			f:    policyUpload,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"policyDelete": {
			f:    policyDelete,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"policyGet": {
			f:    policyGet,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"policyList": {
			f:    policyList,
			args: []string{"foo"},
			err:  errorInvalidArgCount(1, 0, []string{"foo"}),
		},
		"volumeCreate": {
			f:    volumeCreate,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"volumeCreateInvalidPolicy": {
			f:    volumeCreate,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<policyName>/<volumeName>`),
		},
		"volumeGet": {
			f:    volumeGet,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"volumeGetInvalidPolicy": {
			f:    volumeGet,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<policyName>/<volumeName>`),
		},
		"volumeForceRemove": {
			f:    volumeForceRemove,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"volumeForceRemoveInvalidPolicy": {
			f:    volumeForceRemove,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<policyName>/<volumeName>`),
		},
		"volumeRemove": {
			f:    volumeRemove,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"volumeRemoveInvalidPolicy": {
			f:    volumeRemove,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<policyName>/<volumeName>`),
		},
		"volumeList": {
			f:    volumeList,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"volumeListAll": {
			f:    volumeListAll,
			args: []string{"foo"},
			err:  errorInvalidArgCount(1, 0, []string{"foo"}),
		},
		"useList": {
			f:    useList,
			args: []string{"foo"},
			err:  errorInvalidArgCount(1, 0, []string{"foo"}),
		},
		"useGet": {
			f:    useGet,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"useGetInvalidPolicy": {
			f:    useGet,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<policyName>/<volumeName>`),
		},
		"useTheForce": {
			f:    useTheForce,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"useTheForceInvalidPolicy": {
			f:    useTheForce,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<policyName>/<volumeName>`),
		},
	}

	for key, test := range testMap {
		fs := flag.NewFlagSet("test", flag.PanicOnError)
		c.Assert(fs.Parse(test.args), IsNil)
		ctx := cli.NewContext(nil, fs, nil)
		_, err := test.f(ctx)
		c.Assert(err, DeepEquals, test.err, Commentf("test key: %q", key))
	}
}
