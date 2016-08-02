package mount

import (
	"fmt"
	"sync"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage"

	log "github.com/Sirupsen/logrus"
)

// Collection is a data structure used for tracking live mounts.
type Collection struct {
	mountMap      map[string]*storage.Mount
	mountMapMutex sync.Mutex
}

// NewCollection properly initializes the Collection struct.
func NewCollection() *Collection {
	return &Collection{
		mountMap: map[string]*storage.Mount{},
	}
}

// Add adds a mount to the collection. It is assumed that volplugin will manage
// the pre-existence of a live mount for the purposes of this function and this
// function WILL panic w/ error if given an existing mount as a duplicate.
func (c *Collection) Add(mc *storage.Mount) {
	c.mountMapMutex.Lock()
	defer c.mountMapMutex.Unlock()
	log.Infof("Adding mount %q", mc.Volume.Name)

	if _, ok := c.mountMap[mc.Volume.Name]; ok {
		// we should NEVER see this and volplugin should absolutely crash if it is seen.
		panic(fmt.Sprintf("Mount for %q already existed!", mc.Volume.Name))
	}

	c.mountMap[mc.Volume.Name] = mc
}

// Remove removes a mount from the mount collection.
func (c *Collection) Remove(vol string) {
	c.mountMapMutex.Lock()
	defer c.mountMapMutex.Unlock()
	log.Infof("Removing mount %q", vol)
	delete(c.mountMap, vol)
}

// Get obtains the mount from the collection
func (c *Collection) Get(vol string) (*storage.Mount, error) {
	c.mountMapMutex.Lock()
	defer c.mountMapMutex.Unlock()
	log.Debugf("Retrieving mount %q", vol)
	log.Debugf("Mount collection: %#v", c.mountMap)
	mc, ok := c.mountMap[vol]
	if !ok {
		return nil, errored.Errorf("Could not find mount for volume %q", vol).Combine(errors.NotExists)
	}

	return mc, nil
}
