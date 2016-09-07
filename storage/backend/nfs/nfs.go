package nfs

import (
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/storage"
	"github.com/vishvananda/netlink"
)

// Driver is a basic struct for controlling the NFS driver.
type Driver struct {
	mountpath string
}

// BackendName is the name of the driver.
const BackendName = "nfs"

// NewMountDriver constructs a new NFS driver.
func NewMountDriver(mountPath string) (storage.MountDriver, error) {
	return &Driver{mountpath: mountPath}, nil
}

// Name returns the string associated with the storage backed of the driver
func (d *Driver) Name() string { return BackendName }

func (d *Driver) validateConvertOptions(options string) (map[string]string, error) {
	if options == "" {
		return map[string]string{}, nil
	}

	parts := strings.Split(options, ",")
	mapOptions := map[string]string{}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errored.Errorf("Invalid options, syntax must have data between two commas")
		}

		if strings.HasPrefix(part, "=") || strings.HasSuffix(part, "=") {
			return nil, errored.Errorf("key=value options must contain both a key and a value")
		}

		keyval := strings.Split(part, "=")
		if len(keyval) > 2 || len(keyval) < 1 {
			return nil, errored.Errorf("Option syntax is `key`, or `key=value`. Invalid parameters detected.")
		}

		if len(keyval) == 2 {
			mapOptions[keyval[0]] = keyval[1]
		} else {
			mapOptions[keyval[0]] = ""
		}
	}

	return mapOptions, nil
}

// this converts a hash of options into a string we pass to the mount syscall.
func (d *Driver) mapOptionsToString(mapOpts map[string]string) string {
	ret := ""
	for key, val := range mapOpts {
		ret += key
		if val != "" {
			ret += fmt.Sprintf("=%s", val)
		}
		ret += ","
	}

	return ret[:len(ret)-1] // XXX strip the trailing comma
}

func (d *Driver) mkOpts(do storage.DriverOptions) (string, error) {
	var options string
	if err := do.Volume.Params.Get("options", &options); err != nil {
		return "", err
	}

	mapOpts, err := d.validateConvertOptions(options)
	if err != nil {
		return "", err
	}

	var host string

	if !strings.Contains(do.Source, ":") {
		res, ok := mapOpts["addr"]
		if !ok || strings.TrimSpace(res) == "" {
			return "", errored.Errorf("No server address was provided. Either provide `host:mount` syntax for the source, or set `addr` in the driver options.")
		}

		host = res
	} else {
		parts := strings.SplitN(do.Source, ":", 2)
		if len(parts) < 2 {
			return "", errored.Errorf("Internal error handling server address: %q is invalid, but got through other validation", do.Source)
		}

		host = parts[0]
	}

	if host == "" {
		host = "127.0.0.1"
	}

	if ip := net.ParseIP(host); ip == nil {
		hosts, err := net.LookupHost(host)
		if err != nil {
			return "", errored.Errorf("Host lookup failed for NFS server %q", host).Combine(err)
		}

		if len(hosts) < 1 {
			return "", errored.Errorf("Host lookup for NFS server %q succeeded but returned no address to use", host)
		}
		host = hosts[0]
	}

	mapOpts["addr"] = host

	if err := readNetlink(mapOpts, host); err != nil {
		return "", err
	}

	str := d.mapOptionsToString(mapOpts)

	return fmt.Sprintf("nfsvers=4,%s", str), nil
}

func readNetlink(mapOpts map[string]string, host string) error {
	if _, ok := mapOpts["clientaddr"]; !ok {
		ip := net.ParseIP(host)
		if ip == nil {
			return errored.Errorf("Could not parse IP %q in NFS mount", host)
		}

		list, err := netlink.LinkList()
		if err != nil {
			return errored.Errorf("Error listing netlink interfaces during NFS mount").Combine(err)
		}

		for _, link := range list {
			// XXX for now, at least, we want physical devices only. This keeps us
			//     from accidentally returning virtual interface addresses as the
			//     clientaddr.
			if link.Type() != "device" {
				continue
			}

			addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
			if err != nil {
				return errored.Errorf("Error listing addrs for link %q", link.Attrs().Name)
			}

			for _, addr := range addrs {
				if addr.IPNet.Contains(ip) {
					mapOpts["clientaddr"] = ip.String()
					return nil
				}
			}
		}
	}

	return errored.Errorf("Could not find a suitable clientaddr for mount")
}

// Mount a Volume
func (d *Driver) Mount(do storage.DriverOptions) (*storage.Mount, error) {
	mp, err := d.MountPath(do)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(mp, 0755); err != nil && !os.IsExist(err) {
		return nil, errored.Errorf("Error creating directory %q while preparing NFS mount for %q", mp, do.Source).Combine(err)
	}

	opts, err := d.mkOpts(do)
	if err != nil {
		return nil, err
	}

	times := 0

retry:
	if err := unix.Mount(do.Source, mp, "nfs", 0, opts); err != nil && err != unix.EBUSY {
		if err == unix.EIO {
			logrus.Errorf("I/O error mounting %q Retrying after timeout...", do.Volume.Name)
			time.Sleep(do.Timeout)
			times++
			if times == 3 {
				return nil, errored.Errorf("I/O error mounting %q", do.Volume.Name).Combine(err)
			}

			goto retry
		}

		return nil, errored.Errorf("Error mounting nfs volume %q at %q", do.Source, mp).Combine(err)
	}

	return &storage.Mount{
		Device: do.Source,
		Path:   mp,
		Volume: do.Volume,
	}, nil
}

// Unmount a volume
func (d *Driver) Unmount(do storage.DriverOptions) error {
	mp, err := d.MountPath(do)
	if err != nil {
		return err
	}

	if err := unix.Unmount(mp, 0); err != nil {
		return err
	}

	return nil
}

// internalName translates a volplugin `tenant/volume` name to an internal
// name suitable for the driver. Yields an error if impossible.
func (d *Driver) internalName(volName string) (string, error) {
	// this ensures that the volume contains characters, and that they aren't all
	// just slashes.
	if strings.Replace(volName, "/", "", -1) == "" {
		return "", errored.Errorf("Invalid volume name %q in NFS driver", volName)
	}
	return volName, nil
}

// MountPath describes the path at which the volume should be mounted.
func (d *Driver) MountPath(do storage.DriverOptions) (string, error) {
	return path.Join(d.mountpath, do.Volume.Name), nil
}

// Validate validates the NFS drivers implementation of handling storage.DriverOptions.
func (d *Driver) Validate(do *storage.DriverOptions) error {
	if do.Volume.Name == "" || do.Source == "" {
		return errored.Errorf("No source or volume supplied, cannot mount this volume")
	}

	return nil
}
