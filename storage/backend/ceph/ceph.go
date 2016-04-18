package ceph

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/context"
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

func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) (*executor.ExecResult, error) {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	return executor.New(cmd).Run(ctx)
}

// NewMountDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewMountDriver(mountpath string) (storage.MountDriver, error) {
	return &Driver{mountpath: mountpath}, nil
}

// NewCRUDDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewCRUDDriver() (storage.CRUDDriver, error) {
	return &Driver{}, nil
}

// NewSnapshotDriver is a generator for Driver structs. It is used by the storage
// framework to yield new drivers on every creation.
func NewSnapshotDriver() (storage.SnapshotDriver, error) {
	return &Driver{}, nil
}

// Name returns the ceph backend string
func (c *Driver) Name() string {
	return BackendName
}

func (c *Driver) externalName(s string) string {
	return strings.Join(strings.SplitN(s, ".", 2), "/")
}

// InternalName translates a volplugin `tenant/volume` name to an internal
// name suitable for the driver. Yields an error if impossible.
func (c *Driver) internalName(s string) (string, error) {
	strs := strings.SplitN(s, "/", 2)
	if len(strs) != 2 {
		return "", errored.Errorf("Invalid volume name %q, must be two parts", s)
	}

	if strings.Contains(strs[0], ".") {
		return "", errored.Errorf("Invalid policy name %q, cannot contain '.'", strs[0])
	}

	if strings.Contains(strs[1], "/") {
		return "", errored.Errorf("Invalid volume name %q, cannot contain '/'", strs[1])
	}

	return strings.Join(strs, "."), nil
}

// Create a volume.
func (c *Driver) Create(do storage.DriverOptions) error {
	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return err
	}

	cmd := exec.Command("rbd", "create", intName, "--size", strconv.FormatUint(do.Volume.Size, 10), "--pool", do.Volume.Params["pool"])
	er, err := runWithTimeout(cmd, do.Timeout)

	if er != nil {
		if er.ExitStatus == 17 {
			return storage.ErrVolumeExist
		} else if er.ExitStatus != 0 {
			return errored.Errorf("Creating disk %q: %v", intName, er)
		}
	} else if err != nil {
		return errored.Errorf("Creating Disk: %#v", err)
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
		if err := c.unmapImage(do); err != nil {
			log.Errorf("Error while trying to unmap after failed filesystem creation: %v", err)
		}
		return err
	}

	return c.unmapImage(do)
}

// Destroy a volume.
func (c *Driver) Destroy(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]
	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return err
	}

	cmd := exec.Command("rbd", "snap", "purge", intName, "--pool", poolName)
	er, _ := runWithTimeout(cmd, do.Timeout)
	if er.ExitStatus != 0 {
		return errored.Errorf("Destroying snapshots for disk %q: %v", intName, er.Stderr)
	}

	cmd = exec.Command("rbd", "rm", intName, "--pool", poolName)
	er, _ = runWithTimeout(cmd, do.Timeout)
	if er.ExitStatus != 0 {
		return errored.Errorf("Destroying disk %q: %v (%v)", intName, er, er.Stdout)
	}

	return nil
}

// List all volumes.
func (c *Driver) List(lo storage.ListOptions) ([]storage.Volume, error) {
	poolName := lo.Params["pool"]

retry:
	er, err := executor.NewCapture(exec.Command("rbd", "ls", poolName, "--format", "json")).Run(context.Background())
	if err != nil {
		return nil, err
	}

	if er.ExitStatus != 0 {
		return nil, errored.Errorf("Listing pool %q: %v", poolName, er)
	}

	textList := []string{}

	if err := json.Unmarshal([]byte(er.Stdout), &textList); err != nil {
		log.Errorf("Unmarshalling ls for pool %q: %v. Retrying.", poolName, err)
		time.Sleep(100 * time.Millisecond)
		goto retry
	}

	list := []storage.Volume{}

	for _, name := range textList {
		list = append(list, storage.Volume{Name: c.externalName(strings.TrimSpace(name)), Params: storage.Params{"pool": poolName}})
	}

	return list, nil
}

// Mount a volume. Returns the rbd device and mounted filesystem path.
// If you pass in the params what filesystem to use as `filesystem`, it will
// prefer that to `ext4` which is the default.
func (c *Driver) Mount(do storage.DriverOptions) (*storage.Mount, error) {
	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return nil, err
	}

	// Directory to mount the volume
	volumePath := filepath.Join(c.mountpath, do.Volume.Params["pool"], intName)

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
	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return err
	}

	// Directory to mount the volume
	volumeDir := filepath.Join(c.mountpath, poolName, intName)

	// Unmount the RBD
	var retries int
	var lastErr error

retry:
	if retries < 3 {
		if err := unix.Unmount(volumeDir, 0); err != nil && err != unix.ENOENT && err != unix.EINVAL {
			lastErr = errored.Errorf("Failed to unmount %q (retrying): %v", volumeDir, err)
			log.Error(lastErr)
			retries++
			time.Sleep(100 * time.Millisecond)
			goto retry
		}
	} else {
		return errored.Errorf("Failed to umount after 3 retries").Combine(lastErr.(*errored.Error))
	}

	// Remove the mounted directory
	// FIXME remove all, but only after the FIXME above.
	if err := os.Remove(volumeDir); err != nil && !os.IsNotExist(err) {
		log.Error(errored.Errorf("error removing %q directory: %v", volumeDir, err))
		goto retry
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
	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return err
	}

	snapName = strings.Replace(snapName, " ", "-", -1)
	cmd := exec.Command("rbd", "snap", "create", intName, "--snap", snapName, "--pool", do.Volume.Params["pool"])
	er, err := runWithTimeout(cmd, do.Timeout)
	if err != nil {
		return err
	}

	if er.ExitStatus != 0 {
		return errored.Errorf("Creating snapshot %q (volume %q): %v", snapName, intName, er)
	}

	return nil
}

// RemoveSnapshot removes a named snapshot for the volume. Any error will be returned.
func (c *Driver) RemoveSnapshot(snapName string, do storage.DriverOptions) error {
	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return err
	}

	cmd := exec.Command("rbd", "snap", "rm", intName, "--snap", snapName, "--pool", do.Volume.Params["pool"])
	er, err := runWithTimeout(cmd, do.Timeout)
	if err != nil {
		return err
	}

	if er.ExitStatus != 0 {
		return errored.Errorf("Removing snapshot %q (volume %q): %v", snapName, intName, er)
	}

	return nil
}

// ListSnapshots returns an array of snapshot names provided a maximum number
// of snapshots to be returned. Any error will be returned.
func (c *Driver) ListSnapshots(do storage.DriverOptions) ([]string, error) {
	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("rbd", "snap", "ls", intName, "--pool", do.Volume.Params["pool"])
	ctx, _ := context.WithTimeout(context.Background(), do.Timeout)
	er, err := executor.NewCapture(cmd).Run(ctx)
	if err != nil {
		return nil, err
	}

	if er.ExitStatus != 0 {
		return nil, errored.Errorf("Listing snapshots for (volume %q): %v", intName, er)
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
	intOrigName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return err
	}

	intNewName, err := c.internalName(newName)
	if err != nil {
		return err
	}

	list, err := c.List(storage.ListOptions{Params: storage.Params{"pool": do.Volume.Params["pool"]}})
	for _, vol := range list {
		if intNewName == vol.Name {
			return errored.Errorf("Volume %q already exists", vol.Name)
		}
	}

	errChan := make(chan error, 1)

	cmd := exec.Command("rbd", "snap", "protect", intOrigName, "--snap", snapName, "--pool", do.Volume.Params["pool"])
	er, err := runWithTimeout(cmd, do.Timeout)

	if err != nil {
		errChan <- err
		return err
	}

	defer func() {
		select {
		case err := <-errChan:
			log.Warnf("Error received while copying snapshot: %v. Attempting to cleanup.", err)
			cmd = exec.Command("rbd", "rm", intNewName, "--pool", do.Volume.Params["pool"])
			if er, err := runWithTimeout(cmd, do.Timeout); err != nil || er.ExitStatus != 0 {
				log.Errorf("Error encountered removing new volume %q for volume %q, snapshot %q: %v, %v", intNewName, intOrigName, snapName, err, er.Stderr)
				return
			}
			cmd := exec.Command("rbd", "snap", "unprotect", intOrigName, "--snap", snapName, "--pool", do.Volume.Params["pool"])
			if er, err := runWithTimeout(cmd, do.Timeout); err != nil || er.ExitStatus != 0 {
				log.Errorf("Error encountered unprotecting new volume %q for volume %q, snapshot %q: %v, %v", newName, intOrigName, snapName, err, er.Stderr)
				return
			}
		default:
		}
	}()

	if er.ExitStatus != 0 {
		newerr := errored.Errorf("Protecting snapshot for clone (volume %q, snapshot %q): %v", intOrigName, snapName, err)
		errChan <- newerr
		return newerr
	}

	cmd = exec.Command("rbd", "clone", intOrigName, intNewName, "--snap", snapName, "--pool", do.Volume.Params["pool"])
	er, err = runWithTimeout(cmd, do.Timeout)
	if err != nil {
		errChan <- err
		return err
	}

	if er.ExitStatus != 0 {
		newerr := errored.Errorf("Cloning snapshot to volume (volume %q, snapshot %q): %v", intOrigName, snapName, err)
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

	return mounts, nil
}
