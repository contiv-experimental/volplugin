package entities

import (
	"encoding/json"

	"golang.org/x/net/context"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/client"
	"github.com/contiv/volplugin/errors"

	gojson "github.com/xeipuuv/gojsonschema"
)

// Type definitions for backend drivers
var defaultDrivers = map[string]*BackendDrivers{
	"ceph": {"ceph", "ceph", "ceph"},
	"nfs":  {"", "nfs", ""},
}

var defaultFilesystems = map[string]string{
	"ext4": "mkfs.ext4 -m0 %",
}

var policySchema = gojson.NewStringLoader(PolicySchema)

const defaultFilesystem = "ext4"

// NewPolicyFromJSON constructs a policy directly from a JSON
// blob.
func NewPolicyFromJSON(content []byte) (*Policy, error) {
	policy := &Policy{}
	err := json.Unmarshal(content, policy)
	return policy, err
}

// Path returns the base path of the entity
func (p *Policy) Path(pather *client.Pather) (string, error) {
	if p.Name == "" {
		return "", errors.InvalidPolicy.Combine(errored.New(p.Name))
	}

	path, err := pather.Append("policies", p.Name)
	if err != nil {
		return "", err
	}

	return path.String(), nil
}

// Payload returns the marshalled representation of this policy.
func (p *Policy) Payload() ([]byte, error) {
	content, err := json.Marshal(p)
	if err != nil {
		return nil, errors.MarshalPolicy.Combine(err)
	}

	return content, nil
}

// Validate validates the policy. Returns error if necessary.
func (p *Policy) Validate() error {
	if p.FileSystems == nil {
		p.FileSystems = defaultFilesystems
	}

	if err := p.validateJSON(); err != nil {
		return errors.ErrJSONValidation.Combine(err)
	}

	if p.Backends == nil { // backend should be defined and its validated
		backends, ok := defaultDrivers[p.Backend]

		if !ok {
			return errored.Errorf("Invalid backend: %v", p.Backend)
		}
		p.Backends = backends
	}

	if p.CreateOptions == nil {
		return errors.InvalidPolicy.Combine(errored.New("Missing create options"))
	}

	size, err := p.CreateOptions.ActualSize()
	if err != nil {
		return errored.Errorf("Invalid size").Combine(err)
	}

	if p.Backends.CRUD != "" && size == 0 {
		return errored.Errorf("Size set to zero for non-empty CRUD backend %v", p.Backends.CRUD).Combine(err)
	}

	return nil
}

func (p *Policy) validateJSON() error {
	doc := gojson.NewGoLoader(p)

	if result, err := gojson.Validate(policySchema, doc); err != nil {
		return err
	} else if !result.Valid() {
		return combineErrors(result.Errors())
	}

	if err := p.RuntimeOptions.ValidateJSON(); err != nil {
		return err
	}

	return nil
}

func (p *Policy) String() string {
	return p.Name
}

// NewPolicyClient constructs a *PolicyClient, which conforms to
// client.EntityClient.
func NewPolicyClient(baseClient client.Client) *PolicyClient {
	path, _ := baseClient.Path().Replace("policies") // safe conversion
	return &PolicyClient{client: baseClient, prefix: path}
}

func (pc *PolicyClient) processWatch(node *client.Node, channel chan interface{}) error {
	policy, err := NewPolicyFromJSON(node.Value)
	if err != nil {
		return err
	}

	channel <- policy
	return nil
}

// Watch the policy provided.
func (pc *PolicyClient) Watch(name string) (chan interface{}, chan error, error) {
	path, err := pc.prefix.Append(name)
	if err != nil {
		return nil, nil, err
	}

	channel := make(chan interface{}, 1)
	_, errChan := pc.client.Watch(
		path,
		channel,
		false,
		pc.processWatch,
	)
	return channel, errChan, nil
}

// Publish a policy.
func (pc *PolicyClient) Publish(p *Policy) error {
	if err := p.Validate(); err != nil {
		return err
	}

	value, err := p.Payload()
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
	if err := NewPolicyHistoryClient(pc).Add(p); err != nil {
		return err
	}

	// TODO set volume paths at volume publish time
	// c.client.Set(context.Background(), rootVolume, name, "", &client.SetOptions{Dir: true})

	path, err := pc.prefix.Append(p.Name)
	if err != nil {
		return errors.PublishPolicy.Combine(err)
	}

	if err := pc.client.Set(context.Background(), path, value, nil); err != nil {
		return errors.PublishPolicy.Combine(err)
	}

	return nil
}

// Delete a policy.
func (pc *PolicyClient) Delete(name string) error {
	path, err := pc.prefix.Append(name)
	if err != nil {
		return err
	}

	return pc.client.Delete(context.Background(), path, nil)
}

// Get a policy.
func (pc *PolicyClient) Get(name string) (*Policy, error) {
	if name == "" {
		return nil, errored.Errorf("Policy invalid: empty string for name")
	}

	path, err := pc.prefix.Append(name)
	if err != nil {
		return nil, err
	}

	resp, err := pc.client.Get(context.Background(), path, nil)
	if err != nil {
		return nil, errors.EtcdToErrored(err)
	}

	tc, err := NewPolicyFromJSON(resp.Value)
	if err != nil {
		return nil, err
	}

	return tc, tc.Validate()
}

// Path returns the prefix of the policies.
func (pc *PolicyClient) Path() *client.Pather {
	return pc.prefix
}

// Collection returns the PolicyCollection object which can be
// used to iterate policies.
func (pc *PolicyClient) Collection() *PolicyCollection {
	collection := PolicyCollection(*pc)
	return &collection
}

// WatchAll watches the entire policy tree and returns a chan
// interface{} and a chan error for data passing and error
// reporting respectively.
func (pc *PolicyCollection) WatchAll() (chan interface{}, chan error) {
	channel := make(chan interface{}, 1)
	_, errChan := pc.client.Watch(pc.prefix, channel, true, pc.processWatch)
	return channel, errChan
}

func (pc *PolicyCollection) processWatch(node *client.Node, channel chan interface{}) error {
	policy, err := NewPolicyFromJSON(node.Value)
	if err != nil {
		return err
	}

	channel <- policy

	return nil
}

// List lists all policies.
func (pc *PolicyCollection) List() ([]*Policy, error) {
	val, err := pc.client.Get(context.Background(), pc.prefix, &client.GetOptions{Recursive: true})
	if err != nil {
		return nil, err
	}

	policies := []*Policy{}

	for _, node := range val.Nodes {
		policy, err := NewPolicyFromJSON(node.Value)
		if err != nil {
			return nil, err
		}

		policies = append(policies, policy)
	}

	return policies, nil
}
