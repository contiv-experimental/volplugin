package db

import "time"

// DefaultDrivers are macro type definitions for backend drivers.
var DefaultDrivers = map[string]*BackendDrivers{
	"ceph": {"ceph", "ceph", "ceph"},
	"nfs":  {"", "nfs", ""},
}

// DefaultFilesystems is a map of our default supported filesystems. Overridden
// by policy.
var DefaultFilesystems = map[string]string{
	"ext4": "mkfs.ext4 -m0 %",
}

const (
	// DefaultFilesystem is the default filesystem in volplugin. Overridden by policy.
	DefaultFilesystem = "ext4"
	// GlobalConfigName is the path to the global configuration under the db prefix.
	GlobalConfigName = "global-config"

	// DefaultGlobalTTL is the TTL used when no TTL exists.
	DefaultGlobalTTL = 30 * time.Second
	// DefaultTimeout is the standard command timeout when none is provided.
	DefaultTimeout = 10 * time.Minute

	// TimeoutFixBase is our base timeout -- timeouts supplied to the user are multiplied against this for storage and evaluation.
	TimeoutFixBase = time.Minute
	// TTLFixBase is our base TTL. TTLs supplied to the user are multipled against this for storage and evaulation.
	TTLFixBase = time.Second
	// DefaultMountPath is the standard mount path for all mounts.
	DefaultMountPath = "/mnt"
)

const (
	rootGlobal         = "global-config"
	rootVolume         = "volumes"
	rootRuntimeOptions = "runtime-policies"
	rootPolicy         = "policies"
)
