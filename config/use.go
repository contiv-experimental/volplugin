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

var (
	// UseTypeMount is the string type of mount use locks
	UseTypeMount = "mount"
	// UseTypeSnapshot is the string type of snapshot use locks
	UseTypeSnapshot = "snapshot"
)

// UseMount is the mount locking mechanism for users. Users are hosts,
// consumers of a volume. Examples of uses are: creating a volume, using a
// volume, removing a volume, snapshotting a volume. These are supplied in the
// `Reason` field as text.
type UseMount struct {
	Volume   *VolumeConfig
	Hostname string
	Reason   string
}

// UseSnapshot is similar to UseMount in that it is a locking mechanism, just
// for snapshots this time. Taking snapshots can block certain actions such as
// taking other snapshots or deleting snapshots.
type UseSnapshot struct {
	Volume *VolumeConfig
	Reason string
}

// UseLocker is an interface to locks controlled in etcd, or what we call "users".
type UseLocker interface {
	// GetVolume gets the *VolumeConfig for this use.
	GetVolume() *VolumeConfig
	// GetReason gets the reason for this use.
	GetReason() string
	// Type returns the type of lock.
	Type() string
}

// GetVolume gets the *VolumeConfig for this use.
func (um *UseMount) GetVolume() *VolumeConfig {
	return um.Volume
}

// GetReason gets the reason for this use.
func (um *UseMount) GetReason() string {
	return um.Reason
}

// Type returns the type of lock.
func (um *UseMount) Type() string {
	return UseTypeMount
}

// GetVolume gets the *VolumeConfig for this use.
func (us *UseSnapshot) GetVolume() *VolumeConfig {
	return us.Volume
}

// GetReason gets the reason for this use.
func (us *UseSnapshot) GetReason() string {
	return us.Reason
}

// Type returns the type of lock.
func (us *UseSnapshot) Type() string {
	return UseTypeSnapshot
}

func (c *TopLevelConfig) use(typ string, vc *VolumeConfig) string {
	return c.prefixed(rootUse, typ, vc.String())
}

// PublishUse pushes the use to etcd.
func (c *TopLevelConfig) PublishUse(ut UseLocker) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	_, err = c.etcdClient.Set(context.Background(), c.use(ut.Type(), ut.GetVolume()), string(content), &client.SetOptions{PrevExist: client.PrevNoExist})
	log.Debugf("Publishing use: (error: %v) %#v", err, ut)
	return err
}

// PublishUseWithTTL pushes the use to etcd, with a TTL that expires the record
// if it has not been updated within that time.
func (c *TopLevelConfig) PublishUseWithTTL(ut UseLocker, ttl time.Duration, exist client.PrevExistType) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	log.Debugf("Publishing use with TTL %d: %#v", ttl, ut)

	value := string(content)
	if exist != client.PrevNoExist {
		value = ""
	}

	_, err = c.etcdClient.Set(context.Background(), c.use(ut.Type(), ut.GetVolume()), string(content), &client.SetOptions{TTL: ttl, PrevExist: exist, PrevValue: value})
	return err
}

// RemoveUse will remove a user from etcd. Does not fail if the user does
// not exist.
func (c *TopLevelConfig) RemoveUse(ut UseLocker, force bool) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	log.Debugf("Removing Use Lock: %#v", ut)

	opts := &client.DeleteOptions{PrevValue: string(content)}
	if force {
		opts = nil
	}

	_, err = c.etcdClient.Delete(context.Background(), c.use(ut.Type(), ut.GetVolume()), opts)
	return err
}

// GetUse retrieves the UseMount for the given volume name.
func (c *TopLevelConfig) GetUse(ut UseLocker, vc *VolumeConfig) error {
	resp, err := c.etcdClient.Get(context.Background(), c.use(ut.Type(), vc), nil)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(resp.Node.Value), ut); err != nil {
		return err
	}

	return nil
}

// ListUses lists the items in use.
func (c *TopLevelConfig) ListUses(typ string) ([]string, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.prefixed(rootUse, typ), &client.GetOptions{Sort: true, Recursive: true})
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, node := range resp.Node.Nodes {
		for _, inner := range node.Nodes {
			key := path.Join(strings.TrimPrefix(inner.Key, c.prefixed(rootUse, typ)))
			// trim leading slash
			key = key[1:]
			ret = append(ret, key)
		}
	}

	return ret, nil
}
