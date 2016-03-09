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
	"errors"

	log "github.com/Sirupsen/logrus"

	"github.com/contiv/volplugin/config"
)

var (
	// ErrPublish is an error for when use locks cannot be published
	ErrPublish = errors.New("Could not publish use lock")

	// ErrRemove is an error for when use locks cannot be removed
	ErrRemove = errors.New("Could not remove use lock")
)

const (
	// ReasonCreate is the "create" reason for the lock
	ReasonCreate = "Create"
	// ReasonMount is the "mount" reason for the lock
	ReasonMount = "Mount"
	// ReasonRemove is the "remove" reason for the lock
	ReasonRemove = "Remove"
)

// Driver is the top-level struct for lock objects
type Driver struct {
	Config *config.TopLevelConfig
}

// NewDriver creates a Driver. Requires a configured TopLevelConfig.
func NewDriver(config *config.TopLevelConfig) *Driver {
	return &Driver{Config: config}
}

// ExecuteWithUseLock executes a function within a lock/context of the passed
// *config.UseMount.
func (d *Driver) ExecuteWithUseLock(uc config.UseLocker, runFunc func(d *Driver, uc config.UseLocker) error) error {
	if err := d.Config.PublishUse(uc); err != nil {
		log.Debugf("Could not publish use lock %#v: %v", uc, err)
		return ErrPublish
	}

	defer func() {
		if err := d.Config.RemoveUse(uc, false); err != nil {
			log.Errorf("Could not remove use lock %#v: %v", uc, err)
		}
	}()

	return runFunc(d, uc)
}
