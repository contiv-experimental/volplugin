package volmaster

import (
	"fmt"
	"strings"

	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"
)

const defaultFsCmd = "mkfs.ext4 -m0 %"

func joinVolumeName(config *config.VolumeConfig) string {
	return strings.Join([]string{config.TenantName, config.VolumeName}, ".")
}

func createImage(tenant *config.TenantConfig, config *config.VolumeConfig) error {
	var (
		fscmd string
		ok    bool
	)

	if tenant.FileSystems == nil {
		fscmd = defaultFsCmd
	} else {
		fscmd, ok = tenant.FileSystems[config.Options.FileSystem]
		if !ok {
			return fmt.Errorf("Invalid filesystem %q", config.Options.FileSystem)
		}
	}

	return cephdriver.NewCephDriver().NewVolume(config.Options.Pool, joinVolumeName(config), config.Options.Size).Create(fscmd)
}

func removeImage(config *config.VolumeConfig) error {
	return cephdriver.NewCephDriver().NewVolume(config.Options.Pool, joinVolumeName(config), 0).Remove()
}
