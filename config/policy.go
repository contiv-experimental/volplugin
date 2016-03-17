package config

import (
	"encoding/json"
	"path"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// Policy is the configuration of the policy. It includes default
// information for items such as pool and volume configuration.
type Policy struct {
	DefaultVolumeOptions VolumeOptions     `json:"default-options"`
	FileSystems          map[string]string `json:"filesystems"`
}

// NewPolicy return policy config with specified backend preset
func NewPolicy(backend string) *Policy {
	return &Policy{
		DefaultVolumeOptions: VolumeOptions{
			Backend: backend,
		},
	}
}

var defaultFilesystems = map[string]string{
	"ext4": "mkfs.ext4 -m0 %",
}

const defaultFilesystem = "ext4"

func (c *Client) policy(name string) string {
	return c.prefixed(rootPolicy, name)
}

// PublishPolicy publishes policy intent to the configuration store.
func (c *Client) PublishPolicy(name string, cfg *Policy) error {
	if err := cfg.DefaultVolumeOptions.computeSize(); err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	value, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	// create the volume directory for the policy so that files can be written there.
	// for example: /volplugin/policies/policy1 will create
	// /volplugin/volumes/policy1 so that a volume of policy1/test can be created
	// at /volplugin/volumes/policy1/test
	c.etcdClient.Set(context.Background(), c.prefixed(rootVolume, name), "", &client.SetOptions{Dir: true})

	if _, err := c.etcdClient.Set(context.Background(), c.policy(name), string(value), &client.SetOptions{PrevExist: client.PrevIgnore}); err != nil {
		return err
	}

	return nil
}

// DeletePolicy removes a policy from the configuration store.
func (c *Client) DeletePolicy(name string) error {
	_, err := c.etcdClient.Delete(context.Background(), c.policy(name), nil)
	return err
}

// GetPolicy retrieves a policy from the configuration store.
func (c *Client) GetPolicy(name string) (*Policy, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.policy(name), nil)
	if err != nil {
		return nil, err
	}

	tc := &Policy{}
	if err := json.Unmarshal([]byte(resp.Node.Value), tc); err != nil {
		return nil, err
	}

	err = tc.Validate()
	return tc, err
}

// ListPolicies provides an array of strings corresponding to the name of each
// policy.
func (c *Client) ListPolicies() ([]string, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.prefixed(rootPolicy), &client.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		return nil, err
	}

	policies := []string{}

	for _, node := range resp.Node.Nodes {
		policies = append(policies, path.Base(node.Key))
	}

	return policies, nil
}

// Validate ensures the structure of the policy is sane.
func (cfg *Policy) Validate() error {
	if cfg.FileSystems == nil {
		cfg.FileSystems = defaultFilesystems
	}

	return cfg.DefaultVolumeOptions.Validate()
}
