package config

import (
	"encoding/json"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// Type definitions for backend drivers
var defaultDrivers = map[string]*BackendDrivers{
	"ceph": {"ceph", "ceph", "ceph"},
	"nfs":  {"", "nfs", ""},
}

// Policy is the configuration of the policy. It includes default
// information for items such as pool and volume configuration.
type Policy struct {
	Name           string               `json:"name"`
	Unlocked       bool                 `json:"unlocked,omitempty" merge:"unlocked"`
	CreateOptions  CreateOptions        `json:"create"`
	RuntimeOptions RuntimeOptions       `json:"runtime"`
	DriverOptions  storage.DriverParams `json:"driver"`
	FileSystems    map[string]string    `json:"filesystems"`
	Backends       *BackendDrivers      `json:"backends,omitempty"`
	Backend        string               `json:"backend,omitempty"`
}

// BackendDrivers is a struct containing all the drivers used under this policy
type BackendDrivers struct {
	CRUD     string `json:"crud"`
	Mount    string `json:"mount"`
	Snapshot string `json:"snapshot"`
}

// NewPolicy return policy config with specified backend preset
func NewPolicy() *Policy {
	return &Policy{}
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
	cfg.Name = name

	if err := cfg.Validate(); err != nil {
		return err
	}

	value, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	// NOTE: The creation of the policy revision entry and the actual publishing of the policy
	//       should be wrapped in a transaction so they either both succeed or both fail, but
	//       etcd2 doesn't support transactions (etcd3 does/will).
	//
	//       For now, we create the revision entry first and then publish the policy.  It's
	//       better to have an entry for a policy revision that was never actually published
	//       than to have a policy published which has no revision recorded for it.
	if err := c.CreatePolicyRevision(name, string(value)); err != nil {
		return err
	}

	// create the volume directory for the policy so that files can be written there.
	// for example: /volplugin/policies/policy1 will create
	// /volplugin/volumes/policy1 so that a volume of policy1/test can be created
	// at /volplugin/volumes/policy1/test
	c.etcdClient.Set(context.Background(), c.prefixed(rootVolume, name), "", &client.SetOptions{Dir: true})

	if _, err := c.etcdClient.Set(context.Background(), c.policy(name), string(value), &client.SetOptions{PrevExist: client.PrevIgnore}); err != nil {
		return errors.EtcdToErrored(err)
	}

	return nil
}

// DeletePolicy removes a policy from the configuration store.
func (c *Client) DeletePolicy(name string) error {
	_, err := c.etcdClient.Delete(context.Background(), c.policy(name), nil)
	return errors.EtcdToErrored(err)
}

// GetPolicy retrieves a policy from the configuration store.
func (c *Client) GetPolicy(name string) (*Policy, error) {
	if name == "" {
		return nil, errored.Errorf("Policy invalid: empty string for name")
	}

	resp, err := c.etcdClient.Get(context.Background(), c.policy(name), nil)
	if err != nil {
		return nil, errors.EtcdToErrored(err)
	}

	tc := NewPolicy()
	if err := json.Unmarshal([]byte(resp.Node.Value), tc); err != nil {
		return nil, err
	}

	tc.Name = name

	err = tc.Validate()
	return tc, err
}

// ListPolicies provides an array of strings corresponding to the name of each
// policy.
func (c *Client) ListPolicies() ([]Policy, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.prefixed(rootPolicy), &client.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		return nil, errors.EtcdToErrored(err)
	}

	policies := []Policy{}
	for _, node := range resp.Node.Nodes {
		policy := Policy{}
		if err := json.Unmarshal([]byte(node.Value), &policy); err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}

	return policies, nil
}

// Validate ensures the structure of the policy is sane.
func (cfg *Policy) Validate() error {
	if cfg.FileSystems == nil {
		cfg.FileSystems = defaultFilesystems
	}

	if err := cfg.ValidateJSON(); err != nil {
		return errors.ErrJSONValidation.Combine(err)
	}

	if cfg.Backends == nil { // backend should be defined and its validated
		backends, ok := defaultDrivers[cfg.Backend]

		if !ok {
			return errored.Errorf("Invalid backend: %v", cfg.Backend)
		}
		cfg.Backends = backends
	}

	size, err := cfg.CreateOptions.ActualSize()
	if cfg.Backends.CRUD != "" && (size == 0 || err != nil) {
		return errored.Errorf("Size set to zero for non-empty CRUD backend %v", cfg.Backends.CRUD).Combine(err)
	}

	return nil
}

func (cfg *Policy) String() string {
	return cfg.Name
}
