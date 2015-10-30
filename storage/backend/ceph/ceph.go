package ceph

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/contiv/volplugin/storage"
)

const (
	deviceBase = "/dev/rbd"
	mountBase  = "/mnt/ceph"
)

// Driver implements a ceph backed storage driver for volplugin.
//
// -- Pool naming
//
// All ceph operations require a pool name (specified as `pool`) for them to
// work. Therefore, if no pool is specified, the best error condition will be
// raised.
//
type Driver struct{}

// NewDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewDriver() storage.Driver {
	return &Driver{}
}

// Create a volume.
func (c *Driver) Create(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	ok, err := c.poolExists(poolName)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("Pool %q does not exist", poolName)
	}

	if ok, err := c.Exists(do); ok {
		return fmt.Errorf("Volume %v already exists", do.Volume)
	} else if err != nil {
		return err
	}

	return exec.Command("rbd", "create", do.Volume.Name, "--size", strconv.FormatUint(do.Volume.Size, 10), "--pool", poolName).Run()
}

// Format formats a created volume.
func (c *Driver) Format(do storage.DriverOptions) error {
	device, err := c.mapImage(do)
	if err != nil {
		return err
	}

	defer c.unmapImage(do) // see comments near end of function

	if err := c.mkfsVolume(do.FSOptions.CreateCommand, device); err != nil {
		return err
	}

	// we do this twice so we don't swallow the unmap error if we make it to the
	// return statement
	return c.unmapImage(do)
}

// Destroy a volume.
func (c *Driver) Destroy(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]
	if err := exec.Command("rbd", "snap", "purge", do.Volume.Name, "--pool", poolName).Run(); err != nil {
		return err
	}

	return exec.Command("rbd", "rm", do.Volume.Name, "--pool", poolName).Run()
}

// List all volumes.
func (c *Driver) List(lo storage.ListOptions) ([]storage.Volume, error) {
	poolName := lo.Params["pool"]
	out, err := exec.Command("rbd", "ls", poolName).Output()
	if err != nil {
		return nil, err
	}

	list := []storage.Volume{}
	textList := strings.Split(string(out), "\n")

	for _, name := range textList {
		list = append(list, storage.Volume{Name: strings.TrimSpace(name), Params: storage.Params{"pool": poolName}})
	}

	return list, nil
}

// Mount a volume. Returns the rbd device and mounted filesystem path.
// If you pass in the params what filesystem to use as `filesystem`, it will
// prefer that to `ext4` which is the default.
func (c *Driver) Mount(do storage.DriverOptions) (*storage.Mount, error) {
	// Directory to mount the volume
	volumePath := filepath.Join(mountBase, do.Volume.Params["pool"], do.Volume.Name)

	devName, err := c.mapImage(do)
	if err != nil {
		return nil, err
	}

	// Create directory to mount
	if err := os.MkdirAll(mountBase, 0700); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("error creating %q directory: %v", mountBase, err)
	}

	if err := os.MkdirAll(volumePath, 0700); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("error creating %q directory: %v", volumePath)
	}

	// Obtain the major and minor node information about the device we're mounting.
	// This is critical for tuning cgroups and obtaining metrics for this device only.
	fi, err := os.Stat(devName)
	if err != nil {
		return nil, fmt.Errorf("Failed to stat rbd device %q: %v", devName, err)
	}

	rdev := fi.Sys().(*syscall.Stat_t).Rdev

	major := rdev >> 8
	minor := rdev & 0xFF

	// Mount the RBD
	if err := unix.Mount(devName, volumePath, do.FSOptions.Type, 0, ""); err != nil && err != unix.EBUSY {
		return nil, fmt.Errorf("Failed to mount RBD dev %q: %v", devName, err.Error())
	}

	return &storage.Mount{
		Device:   devName,
		Path:     volumePath,
		Volume:   do.Volume,
		DevMajor: uint(major),
		DevMinor: uint(minor),
	}, nil
}

// Unmount a volume.
func (c *Driver) Unmount(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	// Directory to mount the volume
	volumeDir := filepath.Join(mountBase, poolName, do.Volume.Name)

	// Unmount the RBD
	//
	// MNT_DETACH will make this mountpoint unavailable to new open file requests (at
	// least until it is remounted) but persist for existing open requests. This
	// seems to work well with containers.
	//
	// The checks for ENOENT and EBUSY below are safeguards to prevent error
	// modes where multiple containers will be affecting a single volume.
	// FIXME loop over unmount and ensure the unmount finished before removing dir
	if err := unix.Unmount(volumeDir, unix.MNT_DETACH); err != nil && err != unix.ENOENT {
		return fmt.Errorf("Failed to unmount %q: %v", volumeDir, err)
	}

	// Remove the mounted directory
	// FIXME remove all, but only after the FIXME above.
	if err := os.Remove(volumeDir); err != nil && !os.IsNotExist(err) {
		if err, ok := err.(*os.PathError); ok && err.Err == unix.EBUSY {
			return nil
		}

		return fmt.Errorf("error removing %q directory: %v", volumeDir, err)
	}

	if err := c.unmapImage(do); err != os.ErrNotExist {
		return err
	}

	return nil
}

// Exists returns true if the volume already exists.
func (c *Driver) Exists(do storage.DriverOptions) (bool, error) {
	volumes, err := c.List(storage.ListOptions{Params: do.Volume.Params})
	if err != nil {
		return false, err
	}

	for _, vol := range volumes {
		if vol.Name == do.Volume.Name {
			return true, nil
		}
	}

	return false, nil
}

// CreateSnapshot creates a named snapshot for the volume. Any error will be returned.
func (c *Driver) CreateSnapshot(snapName string, do storage.DriverOptions) error {
	snapName = strings.Replace(snapName, " ", "-", -1)
	return exec.Command("rbd", "snap", "create", do.Volume.Name, "--snap", snapName, "--pool", do.Volume.Params["pool"]).Run()
}

// RemoveSnapshot removes a named snapshot for the volume. Any error will be returned.
func (c *Driver) RemoveSnapshot(snapName string, do storage.DriverOptions) error {
	return exec.Command("rbd", "snap", "rm", do.Volume.Name, "--snap", snapName, "--pool", do.Volume.Params["pool"]).Run()
}

// ListSnapshots returns an array of snapshot names provided a maximum number
// of snapshots to be returned. Any error will be returned.
func (c *Driver) ListSnapshots(do storage.DriverOptions) ([]string, error) {
	out, err := exec.Command("rbd", "snap", "ls", do.Volume.Name, "--pool", do.Volume.Params["pool"]).Output()
	if err != nil {
		return nil, err
	}

	names := []string{}

	lines := strings.Split(string(out), "\n")
	if len(lines) > 1 {
		for _, line := range lines[1:] {
			parts := regexp.MustCompile(`\s+`).Split(line, -1)
			if len(parts) < 3 {
				continue
			}

			names = append(names, parts[2])
		}
	}

	return names, nil
}
