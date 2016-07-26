package config

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/watch"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

func (c *Client) policyArchive(name string) string {
	return c.prefixed(rootPolicyArchive, name)
}

func (c *Client) policyArchiveEntry(name string, timestamp string) string {
	return c.prefixed(rootPolicyArchive, name, timestamp)
}

// CreatePolicyRevision creates an revision entry in a policy's history.
func (c *Client) CreatePolicyRevision(name string, policy string) error {
	timestamp := fmt.Sprint(time.Now().Unix())
	key := c.policyArchiveEntry(name, timestamp)

	_, err := c.etcdClient.Set(context.Background(), key, policy, nil)
	if err != nil {
		return errors.EtcdToErrored(err)
	}

	return nil
}

// ListPolicyRevisions returns a list of all the revisions for a given policy.
func (c *Client) ListPolicyRevisions(name string) ([]string, error) {
	keyspace := c.policyArchive(name)

	resp, err := c.etcdClient.Get(context.Background(), keyspace, &client.GetOptions{Sort: true, Recursive: false, Quorum: true})
	if err != nil {
		return nil, errors.EtcdToErrored(err)
	}

	ret := []string{}

	for _, node := range resp.Node.Nodes {
		_, revision := path.Split(node.Key)

		ret = append(ret, revision)
	}

	return ret, nil
}

// GetPolicyRevision returns a single revision for a given policy.
func (c *Client) GetPolicyRevision(name, revision string) (string, error) {
	keyspace := c.policyArchiveEntry(name, revision)

	resp, err := c.etcdClient.Get(context.Background(), keyspace, &client.GetOptions{Sort: false, Recursive: false, Quorum: true})
	if err != nil {
		return "", errors.EtcdToErrored(err)
	}

	return resp.Node.Value, nil
}

// WatchForPolicyChanges creates a watch which returns the policy name and revision
// whenever a new policy is uploaded.
func (c *Client) WatchForPolicyChanges(activity chan *watch.Watch) {
	keyspace := c.prefixed(rootPolicyArchive)

	w := watch.NewWatcher(activity, keyspace, func(resp *client.Response, w *watch.Watcher) {
		// skip anything not happening _below_ our watch.
		// we don't care about updates happening to the root of our watch here.
		if !strings.HasPrefix(resp.Node.Key, keyspace+"/") {
			return
		}

		s := strings.TrimPrefix(resp.Node.Key, keyspace+"/")

		name, revision := path.Split(s)
		name = name[:len(name)-1] // remove trailing slash

		w.Channel <- &watch.Watch{Key: resp.Node.Key, Config: []string{name, revision}}
	})

	watch.Create(w)
}
