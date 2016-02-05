package config

import (
	"encoding/json"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"

	"golang.org/x/net/context"
)

// UseConfig is the locking mechanism for users. Users are hosts,
// consumers of a volume. Examples of uses are: creating a volume, using a
// volume, removing a volume, snapshotting a volume. These are supplied in the
// `Reason` field as text.
type UseConfig struct {
	Volume   *VolumeConfig
	Hostname string
	Reason   string
}

func (c *TopLevelConfig) use(vc *VolumeConfig) string {
	return c.prefixed(rootUse, vc.String())
}

// PublishUse pushes the use to etcd.
func (c *TopLevelConfig) PublishUse(ut *UseConfig) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	_, err = c.etcdClient.Set(context.Background(), c.use(ut.Volume), string(content), &client.SetOptions{PrevExist: client.PrevNoExist})
	log.Debugf("Publishing use: (error: %v) %#v", err, ut)
	return err
}

// PublishUseWithTTL pushes the use to etcd, with a TTL that expires the record
// if it has not been updated within that time.
func (c *TopLevelConfig) PublishUseWithTTL(ut *UseConfig, ttl time.Duration, exist client.PrevExistType) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	log.Debugf("Publishing use with TTL %d: %#v", ttl, ut)

	value := string(content)
	if exist != client.PrevNoExist {
		value = ""
	}

	_, err = c.etcdClient.Set(context.Background(), c.use(ut.Volume), string(content), &client.SetOptions{TTL: ttl, PrevExist: exist, PrevValue: value})
	return err
}

// RemoveUse will remove a user from etcd. Does not fail if the user does
// not exist.
func (c *TopLevelConfig) RemoveUse(ut *UseConfig, force bool) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	log.Debugf("Removing Use Lock: %#v", ut)

	opts := &client.DeleteOptions{PrevValue: string(content)}
	if force {
		opts = nil
	}

	_, err = c.etcdClient.Delete(context.Background(), c.use(ut.Volume), opts)
	return err
}

// GetUse retrieves the UseConfig for the given volume name.
func (c *TopLevelConfig) GetUse(vc *VolumeConfig) (*UseConfig, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.use(vc), nil)
	if err != nil {
		return nil, err
	}

	ut := &UseConfig{}

	if err := json.Unmarshal([]byte(resp.Node.Value), ut); err != nil {
		return nil, err
	}

	return ut, nil
}

// ListUses lists the items in use.
func (c *TopLevelConfig) ListUses() ([]string, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.prefixed(rootUse), &client.GetOptions{Sort: true, Recursive: true})
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, node := range resp.Node.Nodes {
		for _, inner := range node.Nodes {
			key := path.Join(strings.TrimPrefix(inner.Key, c.prefixed(rootUse)))
			// trim leading slash
			key = key[1:]
			ret = append(ret, key)
		}
	}

	return ret, nil
}
