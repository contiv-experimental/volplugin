package backend

import (
	"fmt"

	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend/ceph"
	"github.com/contiv/volplugin/storage/backend/null"
)

// Drivers is the map of string to storage.Driver.
var Drivers = map[string]func(string) storage.Driver{
	ceph.BackendName: ceph.NewDriver,
	null.BackendName: null.NewDriver,
}

// NewDriver instantiates and return a storage backend instance of the specified type
func NewDriver(backend, mountpath string) (storage.Driver, error) {
	f, ok := Drivers[backend]
	if !ok {
		return nil, fmt.Errorf("invalid driver backend: %q", backend)
	}
	return f(mountpath), nil
}
