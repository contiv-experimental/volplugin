package docker

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	. "testing"

	"github.com/contiv/volplugin/api"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"

	. "gopkg.in/check.v1"
)

type dockerSuite struct {
	client *config.Client
	api    *api.API
	server *httptest.Server
}

var _ = Suite(&dockerSuite{})

func TestDockerAPI(t *T) { TestingT(t) }

func (s *dockerSuite) SetUpTest(c *C) {
	c.Assert(exec.Command("sh", "-c", "set -e; for i in $(rbd ls); do rbd snap purge $i; rbd rm $i; done").Run(), IsNil)
	exec.Command("/bin/sh", "-c", "etcdctl rm --recursive /volplugin").Run()
	client, err := config.NewClient("/volplugin", []string{"http://127.0.0.1:2379"})
	if err != nil {
		c.Fatal(err)
	}

	s.client = client
	global := config.NewGlobalConfig()
	s.api = api.NewAPI(NewVolplugin(), "mon0", client, &global)
	s.server = httptest.NewServer(s.api.Router(s.api))
}

func (s *dockerSuite) postStruct(postPath string, st interface{}) (*http.Response, error) {
	content, err := json.Marshal(st)
	if err != nil {
		return nil, err
	}

	return http.Post(strings.Join([]string{s.server.URL, postPath}, "/"), "application/json", bytes.NewBuffer(content))
}

func (s *dockerSuite) unmarshalResponse(body io.Reader) (*Response, error) {
	resp := &Response{}
	content, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	return resp, json.Unmarshal(content, resp)
}

func (s *dockerSuite) TearDownTest(c *C) {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *dockerSuite) TestBasic(c *C) {
	err := s.client.PublishPolicy("policy1", &config.Policy{
		Name:          "policy1",
		Backend:       "ceph",
		DriverOptions: storage.DriverParams{"pool": "rbd"},
		CreateOptions: config.CreateOptions{
			Size: "10MB",
		},
		RuntimeOptions: config.RuntimeOptions{},
	})
	c.Assert(err, IsNil)

	resp, err := s.postStruct("VolumeDriver.Create", map[string]string{})
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200, Commentf("%v", resp))

	dockerResp, err := s.unmarshalResponse(resp.Body)
	c.Assert(err, IsNil)
	c.Assert(dockerResp.Err, Not(Equals), "", Commentf("%v", dockerResp))
	c.Assert(dockerResp.Mountpoint, Equals, "", Commentf("%v", dockerResp))

	resp, err = s.postStruct("VolumeDriver.Create", nil)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200, Commentf("%v", resp))

	dockerResp, err = s.unmarshalResponse(resp.Body)
	c.Assert(err, IsNil)
	c.Assert(dockerResp.Err, Not(Equals), "", Commentf("%v", dockerResp))
	c.Assert(dockerResp.Mountpoint, Equals, "", Commentf("%v", dockerResp))

	resp, err = s.postStruct("VolumeDriver.Create", VolumeGetRequest{Name: "policy1/test"})
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200, Commentf("%v", resp))

	dockerResp, err = s.unmarshalResponse(resp.Body)
	c.Assert(err, IsNil)
	c.Assert(dockerResp.Err, Equals, "", Commentf("%v", dockerResp))
	c.Assert(dockerResp.Mountpoint, Equals, "", Commentf("%v", dockerResp))

	vol, err := s.client.GetVolume("policy1", "test")
	c.Assert(err, IsNil)
	c.Assert(vol.String(), Equals, "policy1/test")

	out, err := exec.Command("rbd", "ls").CombinedOutput()
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(out), "policy1.test"), Equals, true, Commentf("%v", out))

	resp, err = s.postStruct("VolumeDriver.List", nil)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200, Commentf("%v", resp))
	content, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	list := &VolumeList{}
	c.Assert(json.Unmarshal(content, list), IsNil)
	c.Assert(list.Err, Equals, "", Commentf("%v", list.Err))
	c.Assert(list.Volumes, NotNil)
	c.Assert(len(list.Volumes), Equals, 1)
	c.Assert(list.Volumes[0].Name, Equals, "policy1/test")

	// XXX ensure that remove does NOT remove the volume
	resp, err = s.postStruct("VolumeDriver.Remove", &Volume{Name: "policy1/test"})
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200, Commentf("%v", resp))

	out, err = exec.Command("rbd", "ls").CombinedOutput()
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(out), "policy1.test"), Equals, true, Commentf("%v", out))

	resp, err = s.postStruct("VolumeDriver.List", nil)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200, Commentf("%v", resp))
	content, err = ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	list = &VolumeList{}
	c.Assert(json.Unmarshal(content, list), IsNil)
	c.Assert(list.Err, Equals, "", Commentf("%v", list.Err))
	c.Assert(list.Volumes, NotNil)
	c.Assert(len(list.Volumes), Equals, 1)
	c.Assert(list.Volumes[0].Name, Equals, "policy1/test")

	c.Assert(s.client.RemoveVolume("policy1", "test"), IsNil)

	// XXX config lib cannot remove the literal rbd volume so we just test the
	// API responses here.

	resp, err = s.postStruct("VolumeDriver.List", nil)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200, Commentf("%v", resp))
	content, err = ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	list = &VolumeList{}
	c.Assert(json.Unmarshal(content, list), IsNil)
	c.Assert(list.Err, Equals, "", Commentf("%v", list.Err))
	c.Assert(list.Volumes, IsNil)
}
