package volplugin

import (
	"fmt"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/storage"
)

func (dc *DaemonConfig) addMount(mc *storage.Mount) {
	dc.mountMapMutex.Lock()
	defer dc.mountMapMutex.Unlock()
	if _, ok := dc.mountMap[mc.Volume.Name]; ok {
		// we should NEVER see this and volplugin should absolutely crash if it is seen.
		panic(fmt.Sprintf("Mount for %q already existed!", mc.Volume.Name))
	}

	dc.mountMap[mc.Volume.Name] = mc
}

func (dc *DaemonConfig) removeMount(vol string) {
	dc.mountMapMutex.Lock()
	defer dc.mountMapMutex.Unlock()
	delete(dc.mountMap, vol)
}

func (dc *DaemonConfig) getMount(vol string) (*storage.Mount, error) {
	dc.mountMapMutex.Lock()
	defer dc.mountMapMutex.Unlock()
	val, ok := dc.mountMap[vol]
	if !ok {
		return nil, errored.Errorf("Could not find mount for volume %q", vol).Combine(errors.NotExists)
	}

	return val, nil
}
