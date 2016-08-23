package config

import (
	"encoding/json"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
	"github.com/coreos/etcd/client"

	"golang.org/x/net/context"
)

var (
	// UseTypeMount is the string type of mount use locks
	UseTypeMount = "mount"
	// UseTypeSnapshot is the string type of snapshot use locks
	UseTypeSnapshot = "snapshot"

	// UseTypeVolsupervisor is for taking locks on the volsupervisor process.
	// Please see the UseVolsupervisor type.
	UseTypeVolsupervisor = "volsupervisor"
)

// UseVolsupervisor is a global lock on the volsupervisor process itself.
// UseVolsupervisor is kind of a hack currently and this will be addressed in
// the DB rewrite.
type UseVolsupervisor struct {
	Hostname string
}

// UseMount is the mount locking mechanism for users. Users are hosts,
// consumers of a volume. Examples of uses are: creating a volume, using a
// volume, removing a volume, snapshotting a volume. These are supplied in the
// `Reason` field as text.
type UseMount struct {
	Volume   string
	Hostname string
	Reason   string
}

// UseSnapshot is similar to UseMount in that it is a locking mechanism, just
// for snapshots this time. Taking snapshots can block certain actions such as
// taking other snapshots or deleting snapshots.
type UseSnapshot struct {
	Volume string
	Reason string
}

// UseLocker is an interface to locks controlled in etcd, or what we call "users".
type UseLocker interface {
	// GetVolume gets the volume name for this use.
	GetVolume() string
	// GetReason gets the reason for this use.
	GetReason() string
	// Type returns the type of lock.
	Type() string
	// MayExist determines if a key may exist during initial write
	MayExist() bool
}

// GetVolume gets the *Volume for this use.
func (um *UseMount) GetVolume() string {
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

// MayExist determines if a key may exist during initial write
func (um *UseMount) MayExist() bool {
	return true
}

// GetVolume gets the *Volume for this use.
func (us *UseSnapshot) GetVolume() string {
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

// MayExist determines if a key may exist during initial write
func (us *UseSnapshot) MayExist() bool {
	return false
}

// GetVolume returns a static string so it can be a singleton here.
func (us *UseVolsupervisor) GetVolume() string {
	return "volsupervisor"
}

// GetReason gets the reason for this use.
func (us *UseVolsupervisor) GetReason() string {
	return ""
}

// Type returns the type of lock.
func (us *UseVolsupervisor) Type() string {
	return UseTypeVolsupervisor
}

// MayExist determines if a key may exist during initial write
func (us *UseVolsupervisor) MayExist() bool {
	return false
}

func (c *Client) use(typ string, vc string) string {
	return c.prefixed(rootUse, typ, vc)
}

// PublishUse pushes the use to etcd.
func (c *Client) PublishUse(ut UseLocker) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	_, err = c.etcdClient.Set(context.Background(), c.use(ut.Type(), ut.GetVolume()), string(content), &client.SetOptions{PrevExist: client.PrevNoExist})
	if _, ok := err.(client.Error); ok && err.(client.Error).Code == client.ErrorCodeNodeExist {
		if ut.MayExist() {
			_, err := c.etcdClient.Set(context.Background(), c.use(ut.Type(), ut.GetVolume()), string(content), &client.SetOptions{PrevExist: client.PrevExist, PrevValue: string(content)})
			return errors.EtcdToErrored(err)
		}
		return errors.Exists.Combine(err)
	}

	logrus.Debugf("Publishing use: (error: %v) %#v", err, ut)
	return errors.EtcdToErrored(err)
}

// PublishUseWithTTL pushes the use to etcd, with a TTL that expires the record
// if it has not been updated within that time.
func (c *Client) PublishUseWithTTL(ut UseLocker, ttl time.Duration) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	if ttl < 0 {
		err := errored.Errorf("TTL was less than 0 for locker %#v!!!! This should not happen!", ut)
		logrus.Error(err)
		return err
	}

	logrus.Debugf("Publishing use with TTL %v: %#v", ttl, ut)
	value := string(content)

	// attempt to set the lock. If the lock cannot be set and it is is empty, attempt to set it now.
	_, err = c.etcdClient.Set(context.Background(), c.use(ut.Type(), ut.GetVolume()), string(content), &client.SetOptions{TTL: ttl, PrevValue: value})
	if err != nil {
		if er, ok := errors.EtcdToErrored(err).(*errored.Error); ok && er.Contains(errors.NotExists) {
			_, err := c.etcdClient.Set(context.Background(), c.use(ut.Type(), ut.GetVolume()), string(content), &client.SetOptions{TTL: ttl, PrevExist: client.PrevNoExist})
			if err != nil {
				return errors.PublishMount.Combine(err)
			}
		} else {
			return errors.PublishMount.Combine(err)
		}
	}

	return nil
}

// RemoveUse will remove a user from etcd. Does not fail if the user does
// not exist.
func (c *Client) RemoveUse(ut UseLocker, force bool) error {
	content, err := json.Marshal(ut)
	if err != nil {
		return err
	}

	logrus.Debugf("Removing Use Lock: %#v", ut)

	opts := &client.DeleteOptions{PrevValue: string(content)}
	if force {
		opts = nil
	}

	_, err = c.etcdClient.Delete(context.Background(), c.use(ut.Type(), ut.GetVolume()), opts)
	return errors.EtcdToErrored(err)
}

// GetUse retrieves the UseMount for the given volume name.
func (c *Client) GetUse(ut UseLocker, vc *Volume) error {
	resp, err := c.etcdClient.Get(context.Background(), c.use(ut.Type(), vc.String()), nil)
	if err != nil {
		return errors.EtcdToErrored(err)
	}

	if err := json.Unmarshal([]byte(resp.Node.Value), ut); err != nil {
		return err
	}

	return nil
}

// ListUses lists the items in use.
func (c *Client) ListUses(typ string) ([]string, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.prefixed(rootUse, typ), &client.GetOptions{Sort: true, Recursive: true})
	if err != nil {
		return nil, errors.EtcdToErrored(err)
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
