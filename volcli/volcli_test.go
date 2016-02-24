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
		"tenantUpload": {
			f:    tenantUpload,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"tenantDelete": {
			f:    tenantDelete,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"tenantGet": {
			f:    tenantGet,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"tenantList": {
			f:    tenantList,
			args: []string{"foo"},
			err:  errorInvalidArgCount(1, 0, []string{"foo"}),
		},
		"volumeCreate": {
			f:    volumeCreate,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"volumeCreateInvalidTenant": {
			f:    volumeCreate,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<tenantName>/<volumeName>`),
		},
		"volumeGet": {
			f:    volumeGet,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"volumeGetInvalidTenant": {
			f:    volumeGet,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<tenantName>/<volumeName>`),
		},
		"volumeForceRemove": {
			f:    volumeForceRemove,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"volumeForceRemoveInvalidTenant": {
			f:    volumeForceRemove,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<tenantName>/<volumeName>`),
		},
		"volumeRemove": {
			f:    volumeRemove,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"volumeRemoveInvalidTenant": {
			f:    volumeRemove,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<tenantName>/<volumeName>`),
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
		"useGetInvalidTenant": {
			f:    useGet,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<tenantName>/<volumeName>`),
		},
		"useTheForce": {
			f:    useTheForce,
			args: []string{},
			err:  errorInvalidArgCount(0, 1, []string{}),
		},
		"useTheForceInvalidTenant": {
			f:    useTheForce,
			args: []string{"foo"},
			err:  errorInvalidVolumeSyntax("foo", `<tenantName>/<volumeName>`),
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
