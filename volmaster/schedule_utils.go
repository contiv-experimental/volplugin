package volmaster

import (
	"github.com/contiv/go-etcd/etcd"
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
		if conv, ok := err.(*etcd.EtcdError); ok && conv.ErrorCode == 100 {
			// should never be hit because we create it at volmaster boot, but yeah.
			// FIXME this breaks if we start the volmaster before etcd.
			return
		}

		log.Errorf("Runtime configuration incorrect: %v", err)
		return
	}

	for _, tenant := range tenants {
		volumes, err := config.ListVolumes(tenant)
		conv, ok := err.(*etcd.EtcdError)
		if err != nil {
			if ok && conv.ErrorCode == 100 {
				continue
			}

			log.Errorf("Runtime configuration incorrect: %v", err)
			return
		}

		dispatch(&volumeDispatch{config, tenant, volumes})
	}
}
