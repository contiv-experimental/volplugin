package config

import (
	"errors"
	"path"

	"github.com/contiv/volplugin/watch"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

const (
	rootVolume = "volumes"
	rootUse    = "users"
	rootPolicy = "policies"
)

var defaultPaths = []string{rootVolume, rootUse, rootPolicy}

// ErrExist indicates when a key in etcd exits already. Used for create logic.
var ErrExist = errors.New("Already exists")

// Request provides a request structure for communicating with the
// volmaster.
type Request struct {
	Volume  string `json:"volume"`
	Policy  string `json:"policy"`
	Options map[string]string
}

// RequestCreate provides a request structure for creating new volumes.
type RequestCreate struct {
	Policy string            `json:"policy"`
	Volume string            `json:"volume"`
	Opts   map[string]string `json:"opts"`
}

// Client is the top-level struct for communicating with the intent store.
type Client struct {
	etcdClient client.KeysAPI
	prefix     string
}

// NewClient creates a Client struct which can drive communication
// with the configuration store.
func NewClient(prefix string, etcdHosts []string) (*Client, error) {
	etcdCfg := client.Config{
		Endpoints: etcdHosts,
	}

	etcdClient, err := client.New(etcdCfg)
	if err != nil {
		return nil, err
	}

	config := &Client{
		prefix:     prefix,
		etcdClient: client.NewKeysAPI(etcdClient),
	}

	watch.Init(config.etcdClient)

	config.etcdClient.Set(context.Background(), config.prefix, "", &client.SetOptions{Dir: true})
	for _, path := range defaultPaths {
		config.etcdClient.Set(context.Background(), config.prefixed(path), "", &client.SetOptions{Dir: true})
	}

	return config, nil
}

func (c *Client) prefixed(strs ...string) string {
	str := c.prefix
	for _, s := range strs {
		str = path.Join(str, s)
	}

	return str
}
