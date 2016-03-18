package ceph

import (
	"errors"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/storage"
)

var errNotFound = errors.New("Could not find rbd kernel driver entry")

const (
	mountInfoFile  = "/proc/self/mountinfo"
	deviceInfoFile = "/proc/devices"
)

func rbdDevID() (string, error) {
	content, err := ioutil.ReadFile(deviceInfoFile)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	blockDevs := false
	for _, line := range lines {
		if !blockDevs {
			blockDevs = line == "Block devices:"
			continue
		}

		if strings.HasSuffix(line, " rbd") {
			parts := strings.Split(line, " ")
			if len(parts) != 2 {
				return "", errored.Errorf("Invalid input from file %q", deviceInfoFile)
			}

			return parts[0], nil
		}
	}

	return "", errNotFound
}

func getMounts() ([]*storage.Mount, error) {
	mounts := []*storage.Mount{}

	devid, err := rbdDevID()
	if err == errNotFound {
		// XXX we mask this error, because if nothing has been mounted yet the
		// kernel will not display the kernel driver in the devices list, which
		// means we cannot probe, and it's pointless anyways because there will be
		// no mounts to list.
		return mounts, nil
	} else if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadFile(mountInfoFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		parts := strings.Split(line, " ")
		devParts := strings.Split(parts[2], ":")
		if len(devParts) != 2 {
			return nil, errored.Errorf("Could not parse %q properly.", mountInfoFile)
		}

		if devParts[0] == devid {
			devMajor, err := strconv.ParseUint(devParts[0], 10, 64)
			if err != nil {
				return nil, err
			}

			devMinor, err := strconv.ParseUint(devParts[1], 10, 64)
			if err != nil {
				return nil, err
			}

			mounts = append(mounts, &storage.Mount{
				DevMajor: uint(devMajor),
				DevMinor: uint(devMinor),
				Path:     parts[4],
				Device:   parts[9],
				// Deliberately omit the volume which will be merged in later.
			})
		}
	}

	return mounts, nil
}
