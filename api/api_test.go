package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	. "testing"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"

	. "gopkg.in/check.v1"
)

type mockServer http.HandlerFunc

func (m mockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m(w, r)
}

type apiSuite struct {
	api       *API
	server    *httptest.Server
	ServeHTTP func(w http.ResponseWriter, r *http.Request)
}

var _ = Suite(&apiSuite{})

func TestAPI(t *T) { TestingT(t) }

func (a *apiSuite) getVolumeResponse(body io.Reader) (VolumeCreateResponse, error) {
	vr := VolumeCreateResponse{}
	content, err := ioutil.ReadAll(body)
	if err != nil {
		return vr, err
	}
	err = json.Unmarshal(content, &vr)
	return vr, err
}

func (a *apiSuite) uploadPolicy(policyName string) error {
	content, err := ioutil.ReadFile(fmt.Sprintf("../systemtests/testdata/ceph/%s.json", policyName))
	if err != nil {
		return err
	}

	policy := &config.Policy{}
	if err := json.Unmarshal(content, policy); err != nil {
		return err
	}

	if err := a.api.Client.PublishPolicy("policy1", policy); err != nil {
		return err
	}

	return nil
}

func (a *apiSuite) SetUpTest(c *C) {
	exec.Command("sh", "-c", "for i in $(rbd ls); do rbd snap purge $i; rbd rm $i; done").Run()
	exec.Command("etcdctl", "rm", "--recursive", "/volplugin").Run()
	a.server = nil
	client, err := config.NewClient("/volplugin", []string{"http://127.0.0.1:2379"})
	c.Assert(err, IsNil)
	global := config.NewGlobalConfig()
	a.api = &API{Client: client, Global: &global}
}

func (a *apiSuite) TearDownTest(c *C) {
	if a.server != nil {
		a.server.CloseClientConnections()
		a.server.Close()
	}
}

func (a *apiSuite) TestCreate(c *C) {
	// TODO: test body text
	m := mockServer(a.api.Create)
	a.server = httptest.NewServer(m)

	callContent, err := json.Marshal(VolumeCreateRequest{Name: "policy1/test"})
	c.Assert(err, IsNil)
	resp, err := http.Post(a.server.URL, "application/json", bytes.NewBuffer(callContent))
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 500)

	c.Assert(a.uploadPolicy("policy1"), IsNil)

	resp, err = http.Post(a.server.URL, "application/json", bytes.NewBuffer(callContent))
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200)

	out, err := exec.Command("rbd", "ls").CombinedOutput()
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(out), "policy1.test"), Equals, true, Commentf(resp.Status))

	resp, err = http.Post(a.server.URL, "application/json", bytes.NewBuffer(callContent))
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 500)

	resp, err = http.Post(a.server.URL, "application/json", bytes.NewBufferString("[]"))
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 500)

	for _, name := range []string{"invalid", "/foo", "foo/bar/baz", "policy/"} {
		failContent, err := json.Marshal(VolumeCreateRequest{Name: name})
		c.Assert(err, IsNil)
		resp, err = http.Post(a.server.URL, "application/json", bytes.NewBuffer(failContent))
		c.Assert(err, IsNil)
		c.Assert(resp.StatusCode, Equals, 500)
	}
}

func (a *apiSuite) TestCreateDocker(c *C) {
	// test the docker plugin
	a.api = &API{DockerPlugin: true, Client: a.api.Client, Global: a.api.Global}
	m := mockServer(a.api.Create)
	a.server = httptest.NewServer(m)

	callContent, err := json.Marshal(VolumeCreateRequest{Name: "policy1/test"})
	c.Assert(err, IsNil)
	resp, err := http.Post(a.server.URL, "application/json", bytes.NewBuffer(callContent))
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200)

	vr, err := a.getVolumeResponse(resp.Body)
	c.Assert(err, IsNil)
	c.Assert(vr.Err, Not(Equals), "")
	c.Assert(vr.Err, Matches, "Retrieving Policy:.*Does not exist")

	c.Assert(a.uploadPolicy("policy1"), IsNil)
	resp, err = http.Post(a.server.URL, "application/json", bytes.NewBuffer(callContent))
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200)
	vr, err = a.getVolumeResponse(resp.Body)
	c.Assert(err, IsNil)
	c.Assert(vr.Err, Equals, "")

	out, err := exec.Command("rbd", "ls").CombinedOutput()
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(out), "policy1.test"), Equals, true, Commentf(resp.Status))

	resp, err = http.Post(a.server.URL, "application/json", bytes.NewBuffer(callContent))
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200)

	vr, err = a.getVolumeResponse(resp.Body)
	c.Assert(err, IsNil)
	c.Assert(vr.Err, Not(Equals), "")
	c.Assert(vr.Err, Matches, "Already exists")
}

func (a *apiSuite) TestConstructor(c *C) {
	client, err := config.NewClient("/volplugin", []string{"http://127.0.0.1:2379"})
	c.Assert(err, IsNil)

	global := config.NewGlobalConfig()

	api := &API{Client: client, Global: &global, DockerPlugin: true}
	c.Assert(api, DeepEquals, NewAPI(api.Client, api.Global, api.DockerPlugin))
}

func (a *apiSuite) TestErrors(c *C) {
	rec := httptest.NewRecorder()
	DockerHTTPError(rec, errored.New("An error"))
	c.Assert(rec.Code, Equals, http.StatusOK)
	content, err := json.Marshal(VolumeCreateResponse{Err: "An error"})
	c.Assert(err, IsNil)
	c.Assert(strings.TrimSpace(rec.Body.String()), Equals, string(content))

	rec = httptest.NewRecorder()
	RESTHTTPError(rec, nil)
	c.Assert(rec.Code, Equals, http.StatusInternalServerError)
	c.Assert(strings.TrimSpace(rec.Body.String()), Equals, errors.Unknown.Error())

	rec = httptest.NewRecorder()
	RESTHTTPError(rec, errors.LockFailed)
	c.Assert(rec.Code, Equals, http.StatusInternalServerError)
	c.Assert(strings.TrimSpace(rec.Body.String()), Equals, errors.LockFailed.Error())
}
