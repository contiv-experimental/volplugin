package volplugin

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
)

func (dc *DaemonConfig) increaseMount(mp string) int {
	dc.mountMutex.Lock()
	defer dc.mountMutex.Unlock()

	dc.mountCount[mp]++
	log.Debugf("Mount count increased to %d for %q", dc.mountCount[mp], mp)
	return dc.mountCount[mp]
}

func (dc *DaemonConfig) decreaseMount(mp string) int {
	dc.mountMutex.Lock()
	defer dc.mountMutex.Unlock()

	dc.mountCount[mp]--
	log.Debugf("Mount count decreased to %d for %q", dc.mountCount[mp], mp)
	if dc.mountCount[mp] < 0 {
		panic(fmt.Sprintf("Assertion failed while tracking unmount: mount count for %q is less than 0", mp))
	}

	return dc.mountCount[mp]
}
