package helpers

import (
	"strings"
	"sync"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/db/jsonio"
	"github.com/contiv/volplugin/errors"
)

// WatchInfo encapsulates arguments to watch functions for WrapWatch.
type WatchInfo struct {
	Path       string
	Object     db.Entity
	StopChan   chan struct{}
	ReturnChan chan db.Entity
	ErrorChan  chan error
	Recursive  bool
}

// WrapSet wraps set calls in validations, hooks etc. It is intended to be used
// by database implementations of the db.Client interface.
func WrapSet(c db.Client, obj db.Entity, fun func(string, []byte) error) error {
	if err := obj.Validate(); err != nil {
		return err
	}

	if obj.Hooks().PreSet != nil {
		if err := obj.Hooks().PreSet(c, obj); err != nil {
			return err
		}
	}

	content, err := jsonio.Write(obj)
	if err != nil {
		return err
	}

	path, err := obj.Path()
	if err != nil {
		return err
	}

	if err := fun(path, content); err != nil {
		return err
	}

	if obj.Hooks().PostSet != nil {
		if err := obj.Hooks().PostSet(c, obj); err != nil {
			return err
		}
	}

	return nil
}

// WrapGet wraps get calls in a similar fashion to WrapSet. The return of the
// passed function must return a string key + []byte value respectively.
func WrapGet(c db.Client, obj db.Entity, fun func(string) (string, []byte, error)) error {
	if obj.Hooks().PreGet != nil {
		if err := obj.Hooks().PreGet(c, obj); err != nil {
			return errors.EtcdToErrored(err)
		}
	}

	path, err := obj.Path()
	if err != nil {
		return err
	}

	key, value, err := fun(path)
	if err != nil {
		return err
	}

	if err := jsonio.Read(obj, []byte(value)); err != nil {
		return err
	}

	if err := obj.SetKey(TrimPath(c, key)); err != nil {
		return err
	}

	if obj.Hooks().PostGet != nil {
		if err := obj.Hooks().PostGet(c, obj); err != nil {
			return err
		}
	}

	return obj.Validate()
}

// WrapDelete wraps deletes similar in fashion to WrapGet and WrapSet.
func WrapDelete(c db.Client, obj db.Entity, fun func(string) error) error {
	if obj.Hooks().PreDelete != nil {
		if err := obj.Hooks().PreDelete(c, obj); err != nil {
			return errors.EtcdToErrored(err)
		}
	}

	path, err := obj.Path()
	if err != nil {
		return err
	}

	if err := fun(path); err != nil {
		return err
	}

	if obj.Hooks().PostDelete != nil {
		if err := obj.Hooks().PostDelete(c, obj); err != nil {
			return errors.EtcdToErrored(err)
		}
	}

	return nil
}

// TrimPath removes the prefix from a key, and removes leading and trailing slashes.
func TrimPath(c db.Client, key string) string {
	return strings.Trim(strings.TrimPrefix(strings.Trim(key, "/"), c.Prefix()), "/")
}

// WrapWatch wraps watch calls in each client.
func WrapWatch(c db.Client, obj db.Entity, path string, recursive bool, watchers map[string]chan struct{}, mutex *sync.Mutex, fun func(wi WatchInfo)) (chan db.Entity, chan error) {
	mutex.Lock()
	defer mutex.Unlock()

	stopChan := make(chan struct{})
	retChan := make(chan db.Entity)
	errChan := make(chan error)

	wi := WatchInfo{
		Path:       path,
		Object:     obj,
		Recursive:  recursive,
		StopChan:   stopChan,
		ReturnChan: retChan,
		ErrorChan:  errChan,
	}

	go fun(wi)

	_, ok := watchers[path]
	if ok {
		close(watchers[path])
	}
	watchers[path] = stopChan

	return retChan, errChan
}

// WatchStop stops a watch for a given object.
func WatchStop(c db.Client, path string, watchers map[string]chan struct{}, mutex *sync.Mutex) error {
	mutex.Lock()
	defer mutex.Unlock()
	stopChan, ok := watchers[path]
	if !ok {
		return errors.InvalidDBPath.Combine(errored.Errorf("missing key %v during watch", path))
	}

	close(stopChan)
	delete(watchers, path)

	return nil
}

// ReadAndSet reads a json value into a copy of obj, sets the keys and runs any
// hooks. Intended to be used post-watch receive.
func ReadAndSet(c db.Client, obj db.Entity, key string, value []byte) (db.Entity, error) {
	copy := obj.Copy()

	if err := jsonio.Read(copy, []byte(value)); err != nil {
		// This is kept this way so a buggy policy won't break listing all of them
		return nil, errored.Errorf("Received error retrieving value at path %q: %v", key, err)
	}

	if err := copy.SetKey(TrimPath(c, key)); err != nil {
		return nil, err
	}

	// same here. fire hooks to retrieve the full entity. only log but don't append on error.
	if copy.Hooks().PostGet != nil {
		if err := copy.Hooks().PostGet(c, copy); err != nil {
			return nil, errored.Errorf("Error received trying to run fetch hooks during %q list: %v", key, err)
		}
	}

	return copy, nil
}
