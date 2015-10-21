package volmaster

import (
	"github.com/contiv/volplugin/config"

	log "github.com/Sirupsen/logrus"
)

type volumeDispatch struct {
	config  *config.TopLevelConfig
	tenant  string
	volumes map[string]*config.VolumeConfig
}

func iterateVolumes(config *config.TopLevelConfig, dispatch func(v *volumeDispatch)) {
	tenants, err := config.ListTenants()
	if err != nil {
		log.Warn("Could not locate any tenant information; sleeping.")
		return
	}

	for _, tenant := range tenants {
		volumes, err := config.ListVolumes(tenant)
		if err != nil {
			log.Warnf("Could not list volumes for tenant %q: sleeping.", tenant)
			return
		}

		dispatch(&volumeDispatch{config, tenant, volumes})
	}
}
