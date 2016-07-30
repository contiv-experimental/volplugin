package backend

import (
	"os"
	. "testing"
	"time"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend/ceph"
	"github.com/contiv/volplugin/storage/backend/nfs"
)

var filesystems = map[string]storage.FSOptions{
	"ext4": {
		Type:          "ext4",
		CreateCommand: "mkfs.ext4 -m0 %",
	},
}

var volumeSpec = storage.Volume{
	Name:   "test/pithos",
	Size:   10,
	Params: storage.Params{"pool": "rbd"},
}

type backendSuite struct{}

var _ = Suite(&backendSuite{})

func TestBackend(t *T) { TestingT(t) }

func (s *backendSuite) SetUpTest(c *C) {
	if os.Getenv("DEBUG") != "" {
		log.SetLevel(log.DebugLevel)
	}
}

// Tests "New Driver" function
func (s *backendSuite) TestDriverCreation(c *C) {
	c.Assert(NewDriver("", "", "", nil), NotNil)
	c.Assert(NewDriver("badbackend", CRUD, MountPath, nil), NotNil)                // invalid driver backend
	c.Assert(NewDriver(ceph.BackendName, "baddrivertype", MountPath, nil), NotNil) // invalid driver type (CRUD, Moun, Snapshot)
	c.Assert(NewDriver(ceph.BackendName, CRUD, "", nil), NotNil)                   // missing mountpath
	c.Assert(NewDriver(nfs.BackendName, Mount, MountPath, nil), NotNil)            // invalid driver options

	driverOpts := storage.DriverOptions{
		Volume:    volumeSpec,
		FSOptions: filesystems["ext4"],
		Timeout:   5 * time.Second,
	}

	c.Assert(NewDriver(ceph.BackendName, Mount, MountPath, &driverOpts), IsNil)
}
