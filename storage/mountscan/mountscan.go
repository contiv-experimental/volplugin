package mountscan

import (
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
)

const (
	mountInfoFile           = "/proc/self/mountinfo"
	deviceInfoFile          = "/proc/devices"
	nfsMajorID              = 0
	totalMountInfoFieldsNum = 10
)

// GetMountsRequest captures all the params required for scanning mountinfo
type GetMountsRequest struct {
	DriverName   string // ceph, nfs
	FsType       string // nfs4, ext4
	KernelDriver string // rbd, device-mapper, etc.
}

// MountInfo captures the mount info read from /proc/self/mountinfo
type MountInfo struct {
	MountID        uint          // unique mount ID
	ParentID       uint          // ID of the parent mount
	DeviceNumber   *DeviceNumber // Major:Minor device numbers
	Root           string        // path of the directory in the filesystem which is the Root of this mount
	MountPoint     string        // path of the mount point
	MountOptions   string        // per mount options
	OptionalFields string        // in the form of key:value
	Separator      string        // end of optional fields
	FilesystemType string        // e.g ext3, ext4
	MountSource    string
	SuperOptions   string // XXX This field is *not* parsed becuase it is not present on all systems. It is here as a placeholder.
}

// DeviceNumber captures major:minor
type DeviceNumber struct {
	Major uint
	Minor uint
}

// Return major device number of the given kernel driver
func getDevID(kernelDriver string) (uint, error) {
	content, err := ioutil.ReadFile(deviceInfoFile)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(content), "\n")
	blockDevs := false
	for _, line := range lines {
		if !blockDevs {
			blockDevs = line == "Block devices:"
			continue
		}

		if strings.HasSuffix(line, kernelDriver) {
			parts := strings.Split(line, " ")
			if len(parts) != 2 {
				return 0, errored.Errorf("Invalid input from file %q", deviceInfoFile)
			}

			majorID, err := convertToUint(parts[0])
			if err != nil {
				return 0, errored.Errorf("Invalid deviceID %q from device info file for kernel driver %q", parts, kernelDriver).Combine(err)
			}
			return majorID, nil
		}
	}

	return 0, errored.Errorf("Invalid kernel driver: %q", kernelDriver).Combine(errors.ErrDevNotFound)
}

func convertToMountInfo(mountinfo string) (*MountInfo, error) {
	parts := strings.Split(mountinfo, " ")

	mountDetails := &MountInfo{
		Root:           parts[3],
		MountPoint:     parts[4],
		MountOptions:   parts[5],
		OptionalFields: parts[6],
		Separator:      parts[7],
		FilesystemType: parts[8],
		MountSource:    parts[9],
	}

	mountID, err := convertToUint(parts[0])
	if err != nil {
		return nil, err
	}
	mountDetails.MountID = mountID

	parentID, err := convertToUint(parts[1])
	if err != nil {
		return nil, err
	}
	mountDetails.ParentID = parentID

	// Device number details
	mountDetails.DeviceNumber = &DeviceNumber{}
	devParts := strings.Split(parts[2], ":")
	major, err := convertToUint(devParts[0])
	if err != nil {
		return nil, err
	}
	mountDetails.DeviceNumber.Major = major

	minor, err := convertToUint(devParts[1])
	if err != nil {
		return nil, err
	}
	mountDetails.DeviceNumber.Minor = minor

	return mountDetails, nil
}

func getDriverMajorID(request *GetMountsRequest) (uint, error) {
	switch request.DriverName {
	case "nfs":
		return nfsMajorID, nil
	default:
		devID, err := getDevID(request.KernelDriver)
		if err != nil {
			return 0, err
		}
		return devID, nil
	}
}

func validateRequest(request *GetMountsRequest) error {
	if isEmpty(request.DriverName) {
		return errored.Errorf("DriverName is required for scanning mounts")
	}

	switch request.DriverName {
	case "nfs":
		if isEmpty(request.FsType) {
			return errored.Errorf("Filesystem type is required for scanning NFS mounts")
		}
	default:
		if isEmpty(request.KernelDriver) {
			return errored.Errorf("Kernel driver is required for scanning mounts")
		}
	}
	return nil
}

// GetMounts captures the list of mounts that satisfies the given criteria (ceph/nfs/..)
func GetMounts(request *GetMountsRequest) ([]*MountInfo, error) {
	mounts := []*MountInfo{}

	if err := validateRequest(request); err != nil {
		return nil, errors.ErrMountScan.Combine(err)
	}

	driverMajorID, err := getDriverMajorID(request)
	if err != nil {
		return nil, errors.ErrMountScan.Combine(err)
	}

	content, err := ioutil.ReadFile(mountInfoFile)
	if err != nil {
		return nil, errors.ErrMountScan.Combine(err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if !isEmpty(line) {
			if len(strings.Split(line, " ")) < totalMountInfoFieldsNum {
				logrus.Debugf("Insufficient mount info data: %q", line)
				continue
			}
			if mountDetails, err := convertToMountInfo(line); err != nil {
				logrus.Errorf("%s", err)
				continue
			} else {
				if mountDetails.DeviceNumber.Major == driverMajorID {
					if !isEmpty(request.FsType) && mountDetails.FilesystemType != request.FsType {
						continue
					}
					mounts = append(mounts, mountDetails)
				}
			}
		}
	}
	return mounts, nil
}

// Utility functions
func convertToUint(raw string) (uint, error) {
	data, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, errored.Errorf("Invalid data for conversion %q", raw).Combine(err)
	}
	return uint(data), nil
}

func isEmpty(raw string) bool {
	return 0 == len(strings.TrimSpace(raw))
}
