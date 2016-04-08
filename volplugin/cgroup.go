package volplugin

import (
	"fmt"
	"io/ioutil"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/storage"
)

// FIXME find a better place for these
const (
	writeIOPSFile = "/sys/fs/cgroup/blkio/blkio.throttle.write_iops_device"
	readIOPSFile  = "/sys/fs/cgroup/blkio/blkio.throttle.read_iops_device"
	writeBPSFile  = "/sys/fs/cgroup/blkio/blkio.throttle.write_bps_device"
	readBPSFile   = "/sys/fs/cgroup/blkio/blkio.throttle.read_bps_device"
)

func makeLimit(mc *storage.Mount, limit uint64) []byte {
	return []byte(fmt.Sprintf("%d:%d %d\n", mc.DevMajor, mc.DevMinor, limit))
}

func applyCGroupRateLimit(ro config.RuntimeOptions, mc *storage.Mount) error {
	opMap := map[string]uint64{
		writeIOPSFile: uint64(ro.RateLimit.WriteIOPS),
		readIOPSFile:  uint64(ro.RateLimit.ReadIOPS),
		writeBPSFile:  ro.RateLimit.WriteBPS,
		readBPSFile:   ro.RateLimit.ReadBPS,
	}

	for fn, val := range opMap {
		if val > 0 {
			if err := ioutil.WriteFile(fn, makeLimit(mc, val), 0600); err != nil {
				return err
			}
		}
	}

	return nil
}
