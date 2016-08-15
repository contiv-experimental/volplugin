package backend

const SchemaVersionKey = "schema-version"

// Backend defines the interface for a type which implements the functionality to talk
// to a given datastore (etcd2, etcd3, consul, etc.)
type Backend interface {
	// CurrentSchemaVersion returns the version of the last migration which was successfuly run.
	// This is used to ensure that the migration we're about to run hasn't been run already.
	// If the version key does not exist, it will return 0.
	CurrentSchemaVersion() int64

	// CreateDirectory creates a directory at the target path.
	// It acts like mkdir -p, e.g., it won't fail if the directory already exists and
	// can create any depth of nested folder hierarchy.
	// If the target path exists and is a file, it will return an error.
	CreateDirectory(path string) error

	// CreateKey will create a key at the target location.
	// If the target key already exists or is a directory, it will return an error.
	CreateKey(path string, contents []byte) error

	// DeleteDirectory will recursively delete a target directory.
	// If the target directory doesn't exist, it's not considered a failure.
	// If the target directory is a file, it will return an error.
	DeleteDirectory(path string) error

	// DeleteKey will delete the target key if it exists.
	// If the target key doesn't exist, it's not considered a failure.
	// If the target key is a directory, it will refuse to remove it and will return an error.
	DeleteKey(path string) error

	// Name returns the name of the engine ("etcd2", "etcd3", "consul", etc.)
	Name() string

	// UpdateSchemaVersion records the version number of the latest migration which successfully ran.
	UpdateSchemaVersion(version int64) error
}
