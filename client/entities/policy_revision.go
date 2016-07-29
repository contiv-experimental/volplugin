package entities

import (
	"fmt"
	"path"
	"time"

	"github.com/contiv/volplugin/client"
	"golang.org/x/net/context"
)

// NewPolicyHistoryClient returns a client to the policy archive.
func NewPolicyHistoryClient(pc *PolicyClient) *PolicyHistoryClient {
	path, _ := pc.client.Path().Replace("policy-archives")
	return &PolicyHistoryClient{client: pc.client, prefix: path}
}

// Add creates an revision entry in a policy's history.
func (phc *PolicyHistoryClient) Add(policy *Policy) error {
	path, err := phc.prefix.Append(policy.Name)
	if err != nil {
		return err
	}

	if err := phc.client.Set(context.Background(), path, nil, &client.SetOptions{Dir: true}); err != nil {
		return err
	}

	timestamp := fmt.Sprint(time.Now().Unix())
	payload, err := policy.Payload()
	if err != nil {
		return err
	}

	path, err = phc.prefix.Append(policy.Name, timestamp)
	if err != nil {
		return err
	}

	if err := phc.client.Set(context.Background(), path, payload, &client.SetOptions{Exist: client.PrevNoExist}); err != nil {
		return err
	}

	return nil
}

// Get an entry in the policy history.
func (phc *PolicyHistoryClient) Get(name, timestamp string) (*Policy, error) {
	path, err := phc.prefix.Append(name, timestamp)
	if err != nil {
		return nil, err
	}

	resp, err := phc.client.Get(context.Background(), path, nil)
	if err != nil {
		return nil, err
	}

	return NewPolicyFromJSON(resp.Value)
}

// Collection returns a client into the whole tree of archives.
func (phc *PolicyHistoryClient) Collection() *PolicyHistoryCollection {
	return &PolicyHistoryCollection{client: phc.client, prefix: phc.prefix}
}

func (phc *PolicyHistoryCollection) processWatch(node *client.Node, channel chan interface{}) error {
	if node.Dir {
		return nil
	}

	p, err := NewPolicyFromJSON(node.Value)
	if err != nil {
		return err
	}

	channel <- p
	return nil
}

// WatchAll watches for any updates to the policy archive.
func (phc *PolicyHistoryCollection) WatchAll() (chan interface{}, chan error) {
	channel := make(chan interface{}, 1)
	_, errChan := phc.client.Watch(phc.prefix, channel, true, phc.processWatch)
	return channel, errChan
}

// List the archive for a specific policy.
func (phc *PolicyHistoryCollection) List(policy *Policy) (map[string]*Policy, error) {
	pather, err := phc.prefix.Append(policy.Name)
	if err != nil {
		return nil, err
	}

	val, err := phc.client.Get(context.Background(), pather, &client.GetOptions{Recursive: true})
	if err != nil {
		return nil, err
	}

	policies := map[string]*Policy{}

	for _, node := range val.Nodes {
		if node.Dir {
			continue
		}
		policy, err := NewPolicyFromJSON(node.Value)
		if err != nil {
			return nil, err
		}

		policies[path.Base(node.Key)] = policy
	}

	return policies, nil
}
