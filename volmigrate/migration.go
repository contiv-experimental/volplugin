package volmigrate

import (
	"fmt"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/volmigrate/backend"
)

// Migration represents a migration that volmigrate can run.
type Migration struct {
	Version     int64  // each new migration increments this value, must be >= 1
	Description string // e.g., "volplugin 1.3 release"
	runner      func(backend.Backend) error
}

// Run executes the migration using a supplied backend.Backend
func (m *Migration) Run(b backend.Backend) error {
	fmt.Printf("Running migration #%d against %s backend\n", m.Version, b.Name())

	if err := m.runner(b); err != nil {
		return errored.Errorf("Encountered error during migration %d", m.Version).Combine(err)
	}

	if err := b.UpdateSchemaVersion(m.Version); err != nil {
		return errored.Errorf("Successfully applied migration but failed to update schema version key").Combine(err)
	}

	fmt.Println("")

	return nil
}
