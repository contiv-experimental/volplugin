package ceph

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"

	"github.com/contiv/volplugin/storage"
)

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
				return "", fmt.Errorf("Invalid input from file %q", deviceInfoFile)
			}

			return parts[0], nil
		}
	}

	return "", fmt.Errorf("Could not find rbd kernel driver entry")
}

func getMounts() ([]*storage.Mount, error) {
	devid, err := rbdDevID()
	if err != nil {
		return nil, err
	}

	mounts := []*storage.Mount{}

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
			return nil, fmt.Errorf("Could not parse %q properly.", mountInfoFile)
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

func getMapped() ([]*storage.Mount, error) {
	out, err := exec.Command("rbd", "showmapped").Output()
	if err != nil {
		return nil, err
	}

	mounts := []*storage.Mount{}
	for i, line := range strings.Split(string(out), "\n") {
		if i == 0 {
			continue
		}

		parts := spaceSplitRegex.Split(line, -1)
		parts = parts[:len(parts)-1]
		if len(parts) < 5 {
			continue
		}

		mounts = append(mounts, &storage.Mount{
			Device: parts[4],
			Volume: storage.Volume{
				Name: strings.Replace(parts[2], ".", "/", -1),
				Params: map[string]string{
					"pool": parts[1],
				},
			},
		})
	}

	return mounts, nil
}
