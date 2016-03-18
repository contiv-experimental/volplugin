package ceph

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/contiv/errored"
	"github.com/contiv/executor"
	"github.com/contiv/volplugin/storage"

	log "github.com/Sirupsen/logrus"
)

const (
	// BackendName is string for ceph storage backend
	BackendName = "ceph"
)

var spaceSplitRegex = regexp.MustCompile(`\s+`)

// Driver implements a ceph backed storage driver for volplugin.
//
// -- Pool naming
//
// All ceph operations require a pool name (specified as `pool`) for them to
// work. Therefore, if no pool is specified, the best error condition will be
// raised.
//
type Driver struct {
	mountpath string
}

// NewDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewDriver(mountpath string) storage.Driver {
	return &Driver{mountpath: mountpath}
}

// Name returns the ceph backend string
func (c *Driver) Name() string {
	return BackendName
}

// InternalName translates a volplugin `tenant/volume` name to an internal
// name suitable for the driver. Yields an error if impossible.
func (c *Driver) InternalName(s string) (string, error) {
	strs := strings.SplitN(s, "/", 2)
	if strings.Contains(strs[0], ".") {
		return "", errored.Errorf("Invalid tenant name %q, cannot contain '.'", strs[0])
	}

	if strings.Contains(strs[1], "/") {
		return "", errored.Errorf("Invalid volume name %q, cannot contain '/'", strs[0])
	}

	return strings.Join(strs, "."), nil
}

// InternalNameToVolpluginName translates an internal name to a volplugin
// `tenant/volume` syntax name.
func (c *Driver) InternalNameToVolpluginName(s string) string {
	return strings.Join(strings.SplitN(s, ".", 2), "/")
}

// Create a volume.
func (c *Driver) Create(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	ok, err := c.poolExists(poolName)
	if err != nil {
		return err
	}

	if !ok {
		return errored.Errorf("Pool %q does not exist", poolName)
	}

	cmd := exec.Command("rbd", "create", do.Volume.Name, "--size", strconv.FormatUint(do.Volume.Size, 10), "--pool", poolName)
	er, err := executor.NewWithTimeout(cmd, do.Timeout).Run()
	if err != nil {
		return err
	}

	if er.ExitStatus == 4352 {
		return storage.ErrVolumeExist
	} else if er.ExitStatus != 0 {
		return errored.Errorf("Creating disk %q: %v", do.Volume.Name, er)
	}

	return nil
}

// Format formats a created volume.
func (c *Driver) Format(do storage.DriverOptions) error {
	device, err := c.mapImage(do)
	if err != nil {
		return err
	}

	if err := c.mkfsVolume(do.FSOptions.CreateCommand, device, do.Timeout); err != nil {
		c.unmapImage(do)
		return err
	}

	return c.unmapImage(do)
}

// Destroy a volume.
func (c *Driver) Destroy(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	cmd := exec.Command("rbd", "snap", "purge", do.Volume.Name, "--pool", poolName)
	er, err := executor.NewWithTimeout(cmd, do.Timeout).Run()
	if err != nil {
		return err
	}
	if er.ExitStatus != 0 {
		return errored.Errorf("Destroying snapshots for disk %q: %v", do.Volume.Name, er)
	}

	cmd = exec.Command("rbd", "rm", do.Volume.Name, "--pool", poolName)
	er, err = executor.NewWithTimeout(cmd, do.Timeout).Run()
	if err != nil {
		return err
	}

	if er.ExitStatus != 0 {
		return errored.Errorf("Destroying disk %q: %v (%v)", do.Volume.Name, er, er.Stderr)
	}

	return nil
}

// List all volumes.
func (c *Driver) List(lo storage.ListOptions) ([]storage.Volume, error) {
	poolName := lo.Params["pool"]
	er, err := executor.New(exec.Command("rbd", "ls", poolName)).Run()
	if err != nil {
		return nil, err
	}

	if er.ExitStatus != 0 {
		return nil, errored.Errorf("Listing pool %q: %v", poolName, er)
	}

	list := []storage.Volume{}
	textList := strings.Split(er.Stdout, "\n")

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
	volumePath := filepath.Join(c.mountpath, do.Volume.Params["pool"], do.Volume.Name)

	devName, err := c.mapImage(do)
	if err != nil {
		return nil, err
	}

	// Create directory to mount
	if err := os.MkdirAll(c.mountpath, 0700); err != nil && !os.IsExist(err) {
		return nil, errored.Errorf("error creating %q directory: %v", c.mountpath, err)
	}

	if err := os.MkdirAll(volumePath, 0700); err != nil && !os.IsExist(err) {
		return nil, errored.Errorf("error creating %q directory: %v", volumePath, err)
	}

	// Obtain the major and minor node information about the device we're mounting.
	// This is critical for tuning cgroups and obtaining metrics for this device only.
	fi, err := os.Stat(devName)
	if err != nil {
		return nil, errored.Errorf("Failed to stat rbd device %q: %v", devName, err)
	}

	rdev := fi.Sys().(*syscall.Stat_t).Rdev

	major := rdev >> 8
	minor := rdev & 0xFF

	// Mount the RBD
	if err := unix.Mount(devName, volumePath, do.FSOptions.Type, 0, ""); err != nil && err != unix.EBUSY {
		return nil, errored.Errorf("Failed to mount RBD dev %q: %v", devName, err)
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
	volumeDir := filepath.Join(c.mountpath, poolName, do.Volume.Name)

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
		return errored.Errorf("Failed to unmount %q: %v", volumeDir, err)
	}

	// Remove the mounted directory
	// FIXME remove all, but only after the FIXME above.
	if err := os.Remove(volumeDir); err != nil && !os.IsNotExist(err) {
		if err, ok := err.(*os.PathError); ok && err.Err == unix.EBUSY {
			return nil
		}

		return errored.Errorf("error removing %q directory: %v", volumeDir, err)
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
	cmd := exec.Command("rbd", "snap", "create", do.Volume.Name, "--snap", snapName, "--pool", do.Volume.Params["pool"])
	er, err := executor.NewWithTimeout(cmd, do.Timeout).Run()
	if err != nil {
		return err
	}

	if er.ExitStatus != 0 {
		return errored.Errorf("Creating snapshot %q (volume %q): %v", snapName, do.Volume.Name, er)
	}

	return nil
}

// RemoveSnapshot removes a named snapshot for the volume. Any error will be returned.
func (c *Driver) RemoveSnapshot(snapName string, do storage.DriverOptions) error {
	cmd := exec.Command("rbd", "snap", "rm", do.Volume.Name, "--snap", snapName, "--pool", do.Volume.Params["pool"])
	er, err := executor.NewWithTimeout(cmd, do.Timeout).Run()
	if err != nil {
		return err
	}

	if er.ExitStatus != 0 {
		return errored.Errorf("Removing snapshot %q (volume %q): %v", snapName, do.Volume.Name, er)
	}

	return nil
}

// ListSnapshots returns an array of snapshot names provided a maximum number
// of snapshots to be returned. Any error will be returned.
func (c *Driver) ListSnapshots(do storage.DriverOptions) ([]string, error) {
	cmd := exec.Command("rbd", "snap", "ls", do.Volume.Name, "--pool", do.Volume.Params["pool"])
	er, err := executor.NewWithTimeout(cmd, do.Timeout).Run()
	if err != nil {
		return nil, err
	}

	if er.ExitStatus != 0 {
		return nil, errored.Errorf("Listing snapshots for (volume %q): %v", do.Volume.Name, er)
	}

	names := []string{}

	lines := strings.Split(er.Stdout, "\n")
	if len(lines) > 1 {
		for _, line := range lines[1:] {
			parts := spaceSplitRegex.Split(line, -1)
			if len(parts) < 3 {
				continue
			}

			names = append(names, parts[2])
		}
	}

	return names, nil
}

// CopySnapshot copies a snapshot into a new volume. Takes a DriverOptions,
// snap and volume name (string). Returns error on failure.
func (c *Driver) CopySnapshot(do storage.DriverOptions, snapName, newName string) error {
	list, err := c.List(storage.ListOptions{Params: storage.Params{"pool": do.Volume.Params["pool"]}})
	for _, vol := range list {
		if newName == vol.Name {
			return errored.Errorf("Volume %q already exists", vol.Name)
		}
	}

	errChan := make(chan error, 1)

	cmd := exec.Command("rbd", "snap", "protect", do.Volume.Name, "--snap", snapName, "--pool", do.Volume.Params["pool"])
	er, err := executor.NewWithTimeout(cmd, do.Timeout).Run()

	if err != nil {
		errChan <- err
		return err
	}

	defer func() {
		select {
		case err := <-errChan:
			log.Warnf("Error received while copying snapshot: %v. Attempting to cleanup.", err)
			cmd = exec.Command("rbd", "rm", newName, "--pool", do.Volume.Params["pool"])
			if er, err := executor.NewWithTimeout(cmd, do.Timeout).Run(); err != nil || er.ExitStatus != 0 {
				log.Errorf("Error encountered removing new volume %q for volume %q, snapshot %q: %v, %v", newName, do.Volume.Name, snapName, err, er.Stderr)
				return
			}
			cmd := exec.Command("rbd", "snap", "unprotect", do.Volume.Name, "--snap", snapName, "--pool", do.Volume.Params["pool"])
			if er, err := executor.NewWithTimeout(cmd, do.Timeout).Run(); err != nil || er.ExitStatus != 0 {
				log.Errorf("Error encountered unprotecting new volume %q for volume %q, snapshot %q: %v, %v", newName, do.Volume.Name, snapName, err, er.Stderr)
				return
			}
		default:
		}
	}()

	if er.ExitStatus != 0 {
		newerr := errored.Errorf("Protecting snapshot for clone (volume %q, snapshot %q): %v", do.Volume.Name, snapName, err)
		errChan <- newerr
		return newerr
	}

	cmd = exec.Command("rbd", "clone", do.Volume.Name, newName, "--snap", snapName, "--pool", do.Volume.Params["pool"])
	er, err = executor.NewWithTimeout(cmd, do.Timeout).Run()
	if err != nil {
		errChan <- err
		return err
	}

	if er.ExitStatus != 0 {
		newerr := errored.Errorf("Cloning snapshot to volume (volume %q, snapshot %q): %v", do.Volume.Name, snapName, err)
		errChan <- newerr
		return err
	}

	return nil
}

// Mounted describes all the volumes currently mapped on to the host.
func (c *Driver) Mounted(timeout time.Duration) ([]*storage.Mount, error) {
	hostMounts, err := getMounts()
	if err != nil {
		return nil, err
	}

	mapped, err := c.getMapped(timeout)
	if err != nil {
		return nil, err
	}

	if len(hostMounts) != len(mapped) {
		return nil, errored.Errorf("Mounted and mapped volumes do not align.")
	}

	mounts := []*storage.Mount{}

	for _, hostMount := range hostMounts {
		for _, mappedMount := range mapped {
			if hostMount.Device == mappedMount.Device {
				mounts = append(mounts, &storage.Mount{
					Device:   hostMount.Device,
					DevMajor: hostMount.DevMajor,
					DevMinor: hostMount.DevMinor,
					Path:     hostMount.Path,
					Volume:   mappedMount.Volume,
				})
				break
			}
		}
	}

	if len(mounts) != len(hostMounts) || len(mounts) != len(mapped) {
		return nil, errored.Errorf("Did not align all mounts between mapped and mounted volumes.")
	}

	return mounts, nil
}
