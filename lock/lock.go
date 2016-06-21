// Package lock implements coordinated etcd locking across multiple locks to
// provide a safe experience between different parts of the volplugin system.
//
// Currently this package coordinates create, remove, mount and snapshot locks.
// Snapshot locks in particular are special; they are a secondary lock that
// exists for remove operations only.
//
// goroutine-safe functions to manage TTL-safe reporting also exist here.
package lock

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
	"github.com/jbeda/go-wait"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
)

var (
	// Unlocked is a string indicating unlocked operation, this is typically used
	// as a hostname for our locking system.
	Unlocked = "-unlocked-"
)

const (
	// ReasonCreate is the "create" reason for the lock
	ReasonCreate = "Create"
	// ReasonMount is the "mount" reason for the lock
	ReasonMount = "Mount"
	// ReasonRemove is the "remove" reason for the lock
	ReasonRemove = "Remove"

	// ReasonSnapshot is the "snapshot" reason for the lock
	ReasonSnapshot = "Snapshot"
	// ReasonSnapshotPrune is the prune operation for snapshots
	ReasonSnapshotPrune = "SnapshotPrune"

	// ReasonCopy indicates a copy from snapshot operation.
	ReasonCopy = "Copy"
	// ReasonMaintenance indicates that an operator is acquiring the lock.
	ReasonMaintenance = "Maintenance"
)

// Driver is the top-level struct for lock objects
type Driver struct {
	Config *config.Client
}

// NewDriver creates a Driver. Requires a configured Client.
func NewDriver(config *config.Client) *Driver {
	return &Driver{Config: config}
}

// ExecuteWithUseLock executes a function within a lock/context of the passed
// *config.UseMount.
func (d *Driver) ExecuteWithUseLock(uc config.UseLocker, runFunc func(d *Driver, uc config.UseLocker) error) error {
	if err := d.Config.PublishUse(uc); err != nil {
		log.Debugf("Could not publish use lock %#v: %v", uc, err)
		return errors.ErrLockPublish
	}

	defer func() {
		if err := d.Config.RemoveUse(uc, false); err != nil {
			log.Errorf("Could not remove use lock %#v: %v", uc, err)
		}
	}()

	return runFunc(d, uc)
}

// ClearLock removes a lock with a compare/swap first.
func (d *Driver) ClearLock(uc config.UseLocker, timeout time.Duration) error {
	return d.clear(uc, timeout)
}

// ExecuteWithMultiUseLock takes several UseLockers and tries to lock them all
// at the same time. If it fails, it returns an error. If timeout is zero, it
// will not attempt to retry acquiring the lock. Otherwise, it will attempt to
// wait for the provided timeout and only return an error if it fails to
// acquire them in time.
func (d *Driver) ExecuteWithMultiUseLock(ucs []config.UseLocker, timeout time.Duration, runFunc func(d *Driver, ucs []config.UseLocker) error) error {
	acquired := []config.UseLocker{}

	for _, uc := range ucs {
		if err := d.acquire(uc, 0, timeout); err != nil {
			return err
		}
		acquired = append(acquired, uc)
	}

	err := runFunc(d, ucs)

	for _, uc := range acquired {
		if err := d.Config.RemoveUse(uc, false); err != nil {
			log.Errorf("Could not remove use lock %#v: %v", uc, err)
		}
	}

	return err
}

// AcquireWithTTLRefresh accepts a UseLocker, and attempts to acquire the
// lock. When it successfully does, it then spawns a goroutine to refresh the
// lock after a timeout, returning a stop channel. Timeout is jittered to
// mitigate thundering herd problems.
func (d *Driver) AcquireWithTTLRefresh(uc config.UseLocker, ttl, timeout time.Duration) (chan struct{}, error) {
	if err := d.acquire(uc, ttl, timeout); err != nil {
		return nil, err
	}

	stopChan := make(chan struct{})

	go func() {
		for {
			time.Sleep(wait.Jitter(ttl/4, 0))
			select {
			case <-stopChan:
				return
			default:
				if err := d.acquire(uc, ttl, timeout); err != nil {
					log.Errorf("Could not acquire lock %v: %v", uc, err)
					return
				}
			}
		}
	}()

	return stopChan, nil
}

func (d *Driver) lockWait(uc config.UseLocker, timeout time.Duration, now time.Time, reason string) (bool, error) {
	log.Warnf("Could not %s %q lock for %q", reason, uc.GetReason(), uc.GetVolume())
	if timeout != 0 && (timeout == -1 || time.Now().Sub(now) < timeout) {
		log.Warnf("Waiting 100ms for %q lock on %q to free", uc.GetReason(), uc.GetVolume())
		time.Sleep(wait.Jitter(100*time.Millisecond, 0))
		return true, nil
	} else if time.Now().Sub(now) >= timeout {
		return false, errors.LockFailed
	}

	return false, nil
}

func (d *Driver) clear(uc config.UseLocker, timeout time.Duration) error {
	now := time.Now()

retry:
	if err := d.Config.RemoveUse(uc, false); err != nil {
		if ok, err := d.lockWait(uc, timeout, now, "remove"); ok && err == nil {
			goto retry
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (d *Driver) acquire(uc config.UseLocker, ttl, timeout time.Duration) error {
	now := time.Now()

	var err error

retry:
	if ttl != time.Duration(0) {
		if err = d.Config.PublishUseWithTTL(uc, ttl, client.PrevExist); err != nil {
			log.Debugf("Lock publish failed for %q with error: %v. Continuing.", uc, err)
			if err.(*errored.Error).Contains(errors.NotExists) {
				if err = d.Config.PublishUseWithTTL(uc, ttl, client.PrevNoExist); err != nil {
					log.Warnf("Could not acquire %q lock for %q: %v", uc.GetReason(), uc.GetVolume(), err)
				}
			}
		}
	} else {
		if err = d.Config.PublishUse(uc); err != nil {
			log.Warnf("Could not acquire %q lock for %q", uc.GetReason(), uc.GetVolume())
		}
	}

	if err != nil {
		if ok, err := d.lockWait(uc, timeout, now, "publish"); ok && err == nil {
			goto retry
		} else if err != nil {
			return err
		}
	}

	return nil
}
