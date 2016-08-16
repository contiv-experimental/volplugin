package volmigrate

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/volmigrate/backend"
)

var registrationLock sync.Mutex
var availableMigrations map[int64]*Migration
var latestMigrationVersion int64

func init() {
	availableMigrations = make(map[int64]*Migration)
	registerMigrations()
}

// registerMigration makes volmigrate aware of a new migration that can be run.
// If the version number has already been used, it will log a fatal error.
// If the version number is not the next one in the sequence (incremented by 1), it will log a fatal error.
func registerMigration(version int64, description string, f func(backend backend.Backend) error) {
	if version < 1 {
		logrus.Fatalf("Migration versions must be > 0, got: %d\n", version)
	}

	registrationLock.Lock()
	defer registrationLock.Unlock()

	expectedVersion := latestMigrationVersion + 1

	if version != expectedVersion {
		logrus.Fatalf("Expected version '%d', got version '%d'\n", expectedVersion, version)
	}

	_, found := availableMigrations[version]
	if found {
		logrus.Fatalf("Version '%d' is already in use by another migration\n", version)
	}

	m := &Migration{
		Version:     version,
		Description: description,
		runner:      f,
	}

	availableMigrations[version] = m
	latestMigrationVersion = version
}

// If a migration returns an error, volmigrate execution is aborted and the schema version key
// is not updated.  If an operation in a migration can legitimately fail without causing problems,
// don't check the error value from the Create/Delete call.
// Paths are relative to /volplugin and don't require a slash at the beginning:
// e.g., CreateDirectory("foo/bar") will create "/volplugin/foo/bar"
func registerMigrations() {

	// This migration does nothing, but the backend.SchemaVersionKey key will be
	// created and populated after it finishes.
	registerMigration(1, "Create initial schema version key", func(b backend.Backend) error {
		return nil
	})

}
