package volplugin

import (
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
	return dc.mountCount[mp]
}
