package errors

import "github.com/contiv/errored"

// service-level errors
var (
	// Unknown is for those times when we just. don't. know.
	Unknown = errored.New("Unknown error")

	// InvalidPath is a generic error for database issues around pathing.
	InvalidPath = errored.New("Invalid path")

	// Exists is used to exit in situations where duplicated data would be written.
	Exists = errored.New("Already exists")
	// NotExists is used to exit in situations where no data would be read.
	NotExists = errored.New("Does not exist")

	// LockFailed is for when locks fails to acquire.
	LockFailed = errored.New("Locking Operation Failed")

	// LockMismatch is when our compare/swap operations fail.
	LockMismatch = errored.New("Compare/swap lock operation failed. Perhaps it's mounted on a different host?")

	// NoActionTaken signifies that the requested operation was ignored.
	NoActionTaken = errored.New("No action taken")

	// ErrPublish is an error for when use locks cannot be published
	ErrLockPublish = errored.New("Could not publish use lock")

	// ErrRemove is an error for when use locks cannot be removed
	ErrLockRemove = errored.New("Could not remove use lock")

	// VolmasterDown signifies that the apiserver could not be reached.
	VolmasterDown = errored.New("apiserver could not be contacted")
	// VolmasterRequest is used when a request fails.
	VolmasterRequest = errored.New("Making request to apiserver")

	// ErrJSONValidation is used when JSON validation fails
	ErrJSONValidation = errored.New("JSON validation failed")
)

// storage-level errors
var (
	// RateLimit is used when applying rate limiting.
	RateLimit = errored.New("Applying rate limiting configuration")

	// MountPath is used when configuring the mount path.
	MountPath = errored.New("Calculating mount path")

	// SnapshotProtect is used when protecting snapshots for a copy fail.
	SnapshotProtect = errored.New("Protecting snapshot")

	// SnapshotCopy is used when copying snapshots to volumes fail.
	SnapshotCopy = errored.New("Copying snapshot to volume")
)

// protocol-level errors
var (
	// UnmarshalRequest is used when a request failed to decode.
	UnmarshalRequest = errored.New("Unmarshaling Request")
	// MarshalResponse is used when a response failed to encode.
	MarshalResponse = errored.New("Marshalling Response")

	// MarshalGlobal is used when failing to build global configuration
	MarshalGlobal = errored.New("Marshalling global configuration")
	// UnmarshalGlobal is used when failing to unpack global configuration
	UnmarshalGlobal = errored.New("Unmarshalling global configuration")
	// PublishGlobal is used when failing to publish global configuration
	PublishGlobal = errored.New("Publishing global configuration")
	// GetGlobal is used when retriving globals.
	GetGlobal = errored.New("Retrieving global configuration")

	// CannotCopyVolume is used when snapshot copy -> volume operations fail.
	CannotCopyVolume = errored.New("Cannot copy volume")
	// GetVolume is used when retrieving volumes.
	GetVolume = errored.New("Retrieving Volume")
	// InvalidVolume is used both when retrieving volumes and validating the names of volumes.
	InvalidVolume = errored.New("Invalid volume name")
	// RemoveVolume is used when removing volumes.
	RemoveVolume = errored.New("Removing volume")
	// ClearVolume is used when just removing the volume information from etcd.
	ClearVolume = errored.New("Clearing volume records")
	// ListVolume is used when listing volumes.
	ListVolume = errored.New("Retrieving volume list")
	// PublishVolume is used when publishing volumes.
	PublishVolume = errored.New("Publishing Volume")
	// FormatVolume is used when formatting volumes
	FormatVolume = errored.New("Formatting Volume")
	// CreateVolume is used when creating volumes
	CreateVolume = errored.New("Creating Volume")
	// ConfiguringVolume is used when configuring the volume structs.
	ConfiguringVolume = errored.New("Configuring volume parameters")
	// MarshalVolume is used when marshalling volumes.
	MarshalVolume = errored.New("Marshalling volume parameters")
	// UnmarshalVolume is used when unmarshalling volumes.
	UnmarshalVolume = errored.New("Unmarshalling volume parameters")
	// MountSourceRequired is used when a mount source does not exist, but is required.
	MountSourceRequired = errored.New("A mount source does not exist, but is required")

	// UnmarshalRuntime is used when unmarshalling runtime parameters.
	UnmarshalRuntime = errored.New("Unmarshalling runtime parameters")
	// PublishRuntime is used when publishing runtime parameters.
	PublishRuntime = errored.New("Publishing runtime parameters")

	// InvalidPolicy is used both when retrieving policies and validating the names of policies.
	InvalidPolicy = errored.New("Invalid policy name")
	// UnmarshalPolicy is used when unmarshalling policies.
	UnmarshalPolicy = errored.New("Unmarshalling Policy")
	// MarshalPolicy is used when marshalling policies.
	MarshalPolicy = errored.New("Marshalling Policy")
	// GetPolicy is used when retrieving policies.
	GetPolicy = errored.New("Retrieving Policy")
	// ListPolicy is used when listing policies.
	ListPolicy = errored.New("Listing policies")
	// PublishPolicy is used when publishing policies.
	PublishPolicy = errored.New("Publishing policies")
	// ListPolicyRevision is used when listing policy revisions.
	ListPolicyRevision = errored.New("Listing policy revisions")
	// ListPolicyRevision is used when getting a single policy revision.
	GetPolicyRevision = errored.New("Getting policy revision")

	// RemoveImage is used when removing the underlying ceph RBD image.
	RemoveImage = errored.New("Removing image")

	// ListSnapshots is used when listing snapshots.
	ListSnapshots = errored.New("Listing snapshots")
	// SnapshotsUnsupported is used when the backend does not support snapshots.
	SnapshotsUnsupported = errored.New("Backend does not support snapshots")
	// SnapshotFailed is used when failing to take a snapshot.
	SnapshotFailed = errored.New("Failed to take snapshot")
	// MissingSnapshotOption is used when the snapshot option is missing for volume copies.
	MissingSnapshotOption = errored.New("Could not find snapshot option in request, cannot copy.")
	// MissingTargetOption is used when the target option is missing for volume copies.
	MissingTargetOption = errored.New("Could not find target option in request: cannot copy.")

	// RefreshMount is used for the TTL refresher errors.
	RefreshMount = errored.New("Could not refresh mount information")
	// RemoveMount is used when removing a mount.
	RemoveMount = errored.New("Could not remove mount information")
	// PublishMount is used when publishing mounts.
	PublishMount = errored.New("Could not publish mount information")
	// GetMount is used when retrieving mounts.
	GetMount = errored.New("Retrieving mount")
	// MountFailed is used when mounts fail.
	MountFailed = errored.New("Mount failed")
	// UnmountFailed is used when unmounts fail.
	UnmountFailed = errored.New("Unmount failed")

	// GetHostname is used when retreiving the hostname
	GetHostname = errored.New("Retrieving Hostname")
	// GetDriver is used when constructing drivers.
	GetDriver = errored.New("Constructing storage driver")

	// ReadBody is used when reading the request body.
	ReadBody = errored.New("Reading request body")
)
