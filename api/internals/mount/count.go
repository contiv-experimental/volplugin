package mount

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
)

// Counter implements a tracker for specific mounts.
//
// Each volume is assigned an integer and that integer is atomically
// incremented. This is used to track how many mounts a specific volume has on
// a given host. This is used to control mounting and heal the system if
// necessary.
type Counter struct {
	mutex sync.Mutex
	count map[string]int
}

// NewCounter safely constructs a *Counter.
func NewCounter() *Counter {
	return &Counter{
		count: map[string]int{},
	}
}

// Get obtains the mount counter for a volume name.
func (c *Counter) Get(mp string) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.count[mp]
}

// Add increments the mount counter for a volume name and returns the new
// value.
func (c *Counter) Add(mp string) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.count[mp]++
	log.Debugf("Mount count increased to %d for %q", c.count[mp], mp)
	return c.count[mp]
}

// AddCount adds n to the mount counter for a volume name and returns the new
// value.
func (c *Counter) AddCount(mp string, n int) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.count[mp] += n
	log.Debugf("Mount count increased to %d for %q", c.count[mp], mp)
	return c.count[mp]
}

// Sub subtracts from the mount counter and returns the new value. Sub will
// panic if mount counts go less than zero.
func (c *Counter) Sub(mp string) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.count[mp]--
	log.Debugf("Mount count decreased to %d for %q", c.count[mp], mp)
	if c.count[mp] < 0 {
		panic(fmt.Sprintf("Assertion failed while tracking unmount: mount count for %q is less than 0", mp))
	}

	return c.count[mp]
}
