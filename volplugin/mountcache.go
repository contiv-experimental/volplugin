package volplugin

import log "github.com/Sirupsen/logrus"

func (dc *DaemonConfig) mountIncrement(volumeName string) int {
	dc.mountMutex.Lock()
	dc.mountCount[volumeName]++
	retval := dc.mountCount[volumeName]
	dc.mountMutex.Unlock()

	log.Debugf("Increased mount count to %d for %q", retval, volumeName)

	return retval
}

func (dc *DaemonConfig) mountDecrement(volumeName string) int {
	dc.mountMutex.Lock()
	dc.mountCount[volumeName]--
	retval := dc.mountCount[volumeName]
	dc.mountMutex.Unlock()

	log.Debugf("Decreased mount count to %d for %q", retval, volumeName)

	return retval
}
