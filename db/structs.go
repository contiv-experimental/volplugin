package db

import (
	"time"

	"github.com/contiv/volplugin/storage"
)

// Policy is the configuration of the policy. It includes default
// information for items such as pool and volume configuration.
type Policy struct {
	Name           string               `json:"name"`
	Unlocked       bool                 `json:"unlocked,omitempty" merge:"unlocked"`
	CreateOptions  CreateOptions        `json:"create"`
	RuntimeOptions *RuntimeOptions      `json:"runtime"`
	DriverOptions  storage.DriverParams `json:"driver"`
	FileSystems    map[string]string    `json:"filesystems"`
	Backends       *BackendDrivers      `json:"backends,omitempty"`
	Backend        string               `json:"backend,omitempty"`
}

// BackendDrivers is a struct containing all the drivers used under this policy
type BackendDrivers struct {
	CRUD     string `json:"crud"`
	Mount    string `json:"mount"`
	Snapshot string `json:"snapshot"`
}

// VolumeRequest provides a request structure for communicating volumes to the
// apiserver or internally. it is the basic representation of a volume.
type VolumeRequest struct {
	Name    string
	Policy  *Policy
	Options map[string]string
}

// Global is the global configuration.
type Global struct {
	Debug     bool
	Timeout   time.Duration
	TTL       time.Duration
	MountPath string
}

// UseMount is the mount locking mechanism for users. Users are hosts,
// consumers of a volume. Examples of uses are: creating a volume, using a
// volume, removing a volume, snapshotting a volume. These are supplied in the
// `Reason` field as text.
type UseMount struct {
	Volume   string
	Hostname string
	Reason   string
}

// UseSnapshot is similar to UseMount in that it is a locking mechanism, just
// for snapshots this time. Taking snapshots can block certain actions such as
// taking other snapshots or deleting snapshots.
type UseSnapshot struct {
	Volume string
	Reason string
}

// Volume is the configuration of the policy. It includes pool and
// snapshot information.
type Volume struct {
	PolicyName     string               `json:"policy"`
	VolumeName     string               `json:"name"`
	Unlocked       bool                 `json:"unlocked,omitempty" merge:"unlocked"`
	DriverOptions  storage.DriverParams `json:"driver"`
	MountSource    string               `json:"mount" merge:"mount"`
	CreateOptions  CreateOptions        `json:"create"`
	RuntimeOptions *RuntimeOptions      `json:"runtime"`
	Backends       *BackendDrivers      `json:"backends,omitempty"`
}

// CreateOptions are the set of options used by apiserver during the volume
// create operation.
type CreateOptions struct {
	Size       string `json:"size" merge:"size"`
	FileSystem string `json:"filesystem" merge:"filesystem"`
}

// RuntimeOptions are the set of options used by volplugin when mounting the
// volume, and by volsupervisor for calculating periodic work.
type RuntimeOptions struct {
	UseSnapshots bool            `json:"snapshots" merge:"snapshots"`
	Snapshot     SnapshotConfig  `json:"snapshot"`
	RateLimit    RateLimitConfig `json:"rate-limit,omitempty"`

	policyName string
	volumeName string
}

// RateLimitConfig is the configuration for limiting the rate of disk access.
type RateLimitConfig struct {
	WriteBPS uint64 `json:"write-bps" merge:"rate-limit.write.bps"`
	ReadBPS  uint64 `json:"read-bps" merge:"rate-limit.read.bps"`
}

// SnapshotConfig is the configuration for snapshots.
type SnapshotConfig struct {
	Frequency string `json:"frequency" merge:"snapshots.frequency"`
	Keep      uint   `json:"keep" merge:"snapshots.keep"`
}
