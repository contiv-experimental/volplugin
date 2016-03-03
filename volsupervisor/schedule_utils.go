package volsupervisor

import (
	"github.com/contiv/volplugin/config"

	log "github.com/Sirupsen/logrus"
)

type volumeDispatch struct {
	daemonConfig *DaemonConfig
	policy       string
	volumes      map[string]*config.VolumeConfig
}

func (dc *DaemonConfig) iterateVolumes(dispatch func(v *volumeDispatch)) {
	policies, err := dc.Config.ListPolicies()
	if err != nil {
		log.Warnf("Could not locate any policy information; sleeping from error: %v.", err)
		return
	}

	for _, policy := range policies {
		volumes, err := dc.Config.ListVolumes(policy)
		if err != nil {
			log.Warnf("Could not list volumes for policy %q: sleeping.", policy)
			return
		}

		dispatch(&volumeDispatch{dc, policy, volumes})
	}
}
