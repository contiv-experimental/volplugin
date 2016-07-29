package entities

import (
	"time"

	"github.com/contiv/volplugin/client"
)

// Policy is the configuration of the policy. It includes default
// information for items such as pool and volume configuration.
type Policy struct {
	Name           string            `json:"name"`
	Unlocked       bool              `json:"unlocked,omitempty" merge:"unlocked"`
	CreateOptions  *CreateOptions    `json:"create"`
	RuntimeOptions *RuntimeOptions   `json:"runtime"`
	DriverOptions  map[string]string `json:"driver"`
	FileSystems    map[string]string `json:"filesystems"`
	Backends       *BackendDrivers   `json:"backends,omitempty"`
	Backend        string            `json:"backend,omitempty"`
}

// PolicyClient is a client to fetch policies.
type PolicyClient struct {
	client client.Client
	prefix *client.Pather
}

// PolicyCollection is a client.Collection compatible interface to reading
// collections of policies.
type PolicyCollection struct {
	client client.Client
	prefix *client.Pather
}

// PolicyHistoryClient is the client to the policy archives.
type PolicyHistoryClient struct {
	client client.Client
	prefix *client.Pather
}

// PolicyHistoryCollection is a client.HistoryCollection compatible interface
// to reading archives.
type PolicyHistoryCollection struct {
	client client.Client
	prefix *client.Pather
}

// Global is the global configuration.
type Global struct {
	Debug     bool
	Timeout   time.Duration
	TTL       time.Duration
	MountPath string
}

// GlobalClient is a client to fetch globals.
type GlobalClient struct {
	client client.Client
	path   *client.Pather
}

// Volume is the configuration of the policy. It includes pool and
// snapshot information.
type Volume struct {
	PolicyName     string            `json:"policy"`
	VolumeName     string            `json:"name"`
	Unlocked       bool              `json:"unlocked,omitempty" merge:"unlocked"`
	DriverOptions  map[string]string `json:"driver"`
	MountSource    string            `json:"mount" merge:"mount"`
	CreateOptions  *CreateOptions    `json:"create"`
	RuntimeOptions *RuntimeOptions   `json:"runtime"`
	Backends       *BackendDrivers   `json:"backends,omitempty"`
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
	UseSnapshots bool             `json:"snapshots" merge:"snapshots"`
	Snapshot     *SnapshotConfig  `json:"snapshot"`
	RateLimit    *RateLimitConfig `json:"rate-limit,omitempty"`
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

// BackendDrivers is a struct containing all the drivers used under this policy
type BackendDrivers struct {
	CRUD     string `json:"crud"`
	Mount    string `json:"mount"`
	Snapshot string `json:"snapshot"`
}
