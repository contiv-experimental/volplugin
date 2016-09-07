package consul

import (
	"reflect"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/jsonio"
	"github.com/contiv/volplugin/errors"
	"github.com/hashicorp/consul/api"
)

// Acquire and permanently hold a lock.
func (c *Client) Acquire(obj db.Lock) error {
	// these will be cleared (or refreshed) long before they expire so a 15m ttl is a good way to enforce locks without being too aggressive.
	_, err := c.doAcquire(obj, 15*time.Minute, true)
	return err
}

// AcquireWithTTL holds a lock with an expiration TTL. Attempts only once.
func (c *Client) AcquireWithTTL(obj db.Lock, ttl time.Duration) error {
	_, err := c.doAcquire(obj, ttl, false)

	return err
}

// Free a lock. Passing true as the second parameter will force the removal.
func (c *Client) Free(obj db.Lock, force bool) error {
	path, err := obj.Path()
	if err != nil {
		return errors.LockFailed.Combine(err)
	}

	logrus.Debugf("Attempting to free %q by %v", path, obj)

	mylock, ok := c.getLock(path)
	if !ok {
		return errors.LockFailed.Combine(errored.New("Could not locate lock"))
	}

	if !reflect.DeepEqual(obj, mylock.obj) {
		if force {
			goto free
		}

		return errors.LockFailed.Combine(errored.New("invalid lock requested to be freed (wrong host?)"))
	}

free:
	select {
	case <-mylock.monitorChan:
	default:
		mylock.lock.Unlock()
	}

	c.lockMutex.Lock()
	if _, ok := c.locks[path]; ok {
		delete(c.locks, path)
	}
	c.Delete(mylock.obj)
	c.lockMutex.Unlock()

	return nil
}

// AcquireAndRefresh starts a goroutine to refresh the key every 1/4
// (jittered) of the TTL. A stop channel is returned which, when sent a
// struct, will terminate the refresh. Error is returned for anything that
// might occur while setting up the goroutine.
//
// Do not use Free to free these locks, it will not work! Use the stop
// channel.
func (c *Client) AcquireAndRefresh(obj db.Lock, ttl time.Duration) (chan struct{}, error) {
	done, err := c.doAcquire(obj, ttl, true)
	if err != nil {
		return nil, err
	}

	return done, nil
}

func (c *Client) getSession(obj db.Lock, ttl time.Duration, refresh bool) (string, error) {
	var res string
	var err error
	if refresh {
		res, _, err = c.client.Session().Create(&api.SessionEntry{TTL: ttl.String()}, nil)
	} else {
		res, _, err = c.client.Session().CreateNoChecks(&api.SessionEntry{TTL: ttl.String()}, nil)
	}
	return res, err
}

// getLock is a generalization of map access to the locks collection.
func (c *Client) getLock(path string) (*lock, bool) {
	c.lockMutex.Lock()
	tmplock, ok := c.locks[path]
	c.lockMutex.Unlock()

	return tmplock, ok
}

// acquires and yields a *lock{} populated with all the values needed to
// persist and reap this lock. Returns true if it is already running and we
// have re-requested a running lock.
func (c *Client) lock(obj db.Lock, ttl time.Duration, refresh bool) (*lock, bool, error) {
	// consul has a minimum ttl of 10s. Adjust.
	if ttl < 10*time.Second {
		ttl = 10 * time.Second
	}

	path, err := obj.Path()
	if err != nil {
		return nil, false, errors.LockFailed.Combine(err)
	}

	tmplock, ok := c.getLock(path)
	if ok {
		if reflect.DeepEqual(tmplock.obj, obj) {
			return tmplock, true, nil
		}

		return nil, false, errors.LockFailed.Combine(errored.Errorf("Invalid lock attempted at %q -- already exists", path))
	}

	sessionID, err := c.getSession(obj, ttl, refresh)
	if err != nil {
		return nil, false, err
	}

	content, err := jsonio.Write(obj)
	if err != nil {
		return nil, false, err
	}

	mylock, err := c.client.LockOpts(&api.LockOptions{
		LockTryOnce:      true,
		Key:              c.qualified(path),
		Value:            content,
		SessionTTL:       ttl.String(),
		MonitorRetryTime: ttl,
		MonitorRetries:   -1,
	})

	if err != nil {
		return nil, false, err
	}

	stopChan := make(chan struct{})
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(stopChan)
	}()

	monitor, err := mylock.Lock(stopChan)
	if err != nil {
		return nil, false, err
	}

	done := make(chan struct{})

	lockObj := &lock{
		lock:        mylock,
		sessionID:   sessionID,
		doneChan:    done,
		monitorChan: monitor,
		obj:         obj,
		ttl:         ttl,
		path:        path,
		refresh:     refresh,
	}

	return lockObj, false, nil
}

// doRefresh hangs and lets the consul client refresh its key at 1/4 TTL.
func (c *Client) doRefresh(lockObj *lock) {
	if err := c.client.Session().RenewPeriodic((lockObj.ttl / 4).String(), lockObj.sessionID, nil, lockObj.doneChan); err != nil {
		logrus.Errorf("Error during periodic refresh of lock: %v", err)
	}
}

// monitorLock monitors a lock (intended to be run in a goroutine). It blocks
// until it receives one of several signals to terminate the lock refresh, or
// delete the lock if the ttl has expired.
func (c *Client) monitorLock(lockObj *lock) {
	after := make(<-chan time.Time)

	if !lockObj.refresh {
		after = time.After(lockObj.ttl)
	}

	select {
	case <-after:
		lockObj.lock.Unlock()
	case <-lockObj.monitorChan:
	case <-lockObj.doneChan:
		if err := c.Free(lockObj.obj, false); err != nil {
			logrus.Error(err)
		}
	}

	c.lockMutex.Lock()
	lock, ok := c.locks[lockObj.path]
	if ok && reflect.DeepEqual(lock.obj, lockObj.obj) {
		delete(c.locks, lockObj.path)
	}
	c.lockMutex.Unlock()
}

// doAcquire does most of the work for the acquire* class of functions in the
// consul client. See the other private functions in this file for more
// information, but this is usually the top of them all.
func (c *Client) doAcquire(obj db.Lock, ttl time.Duration, refresh bool) (chan struct{}, error) {
	logrus.Debugf("Attempting to acquire %v", obj)

	lockObj, running, err := c.lock(obj, ttl, refresh)
	if err != nil {
		return nil, err
	}

	if running {
		return lockObj.doneChan, nil
	}

	if lockObj.refresh {
		go c.doRefresh(lockObj)
	}

	go c.monitorLock(lockObj)

	c.lockMutex.Lock()
	c.locks[lockObj.path] = lockObj
	c.lockMutex.Unlock()

	logrus.Debugf("Acquired lock for path %v, obj %#v", lockObj.path, lockObj.obj)

	return lockObj.doneChan, nil
}
