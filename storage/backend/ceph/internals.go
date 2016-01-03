package ceph

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/contiv/volplugin/storage"

	log "github.com/Sirupsen/logrus"
)

func (c *Driver) lockImage(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	if err := exec.Command("rbd", "lock", "add", do.Volume.Name, do.Volume.Name, "--pool", poolName).Run(); err != nil {
		log.Debugf("Could not acquire lock for %q: %v", do.Volume.Name, err)
		return err
	}

	return nil
}

func (c *Driver) unlockImage(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	output, err := exec.Command("rbd", "lock", "--format", "json", "list", do.Volume.Name, "--pool", poolName).Output()
	if err != nil {
		return fmt.Errorf("Error running `rbd lock list` for volume %q: %v", do.Volume.Name, err)
	}

	locks := map[string]map[string]string{}

	if err := json.Unmarshal(output, &locks); err != nil {
		return fmt.Errorf("Error unmarshalling lock report for volume %q: %v", do.Volume.Name, err)
	}

	if _, ok := locks[do.Volume.Name]; ok {
		if err := exec.Command("rbd", "lock", "remove", do.Volume.Name, do.Volume.Name, locks[do.Volume.Name]["locker"], "--pool", poolName).Run(); err != nil {
			return fmt.Errorf("Error releasing lock on volume %q: %v", do.Volume.Name, err)
		}
	}

	return nil
}

func (c *Driver) mapImage(do storage.DriverOptions) (string, error) {
	var rbdMaps map[string]struct {
		Pool   string `json:"pool"`
		Name   string `json:"name"`
		Device string `json:"device"`
	}

	poolName := do.Volume.Params["pool"]

	// NOTE: if lockImage() fails, the call to mapImage will try to release the
	// lock before erroring out. It is done there instead of here to reduce code
	// duplication, and avoid creating a very ugly defer statement that relies on
	// top level scoped errors, which really suck.
	if err := c.lockImage(do); err != nil {
		return "", err
	}

	blkdev, err := exec.Command("rbd", "map", do.Volume.Name, "--pool", poolName).Output()
	device := strings.TrimSpace(string(blkdev))

	if err != nil {
		return "", err
	}

	if device == "" {
		output, err := exec.Command("rbd", "showmapped", "--format", "json").Output()

		if err != nil {
			return "", err
		}

		err = json.Unmarshal(output, &rbdMaps)

		if err != nil {
			return "", fmt.Errorf("Could not parse RBD showmapped output: %s", output)
		}

		for i := range rbdMaps {
			if rbdMaps[i].Name == do.Volume.Name && rbdMaps[i].Pool == do.Volume.Params["pool"] {
				device = rbdMaps[i].Device
				break
			}
		}

		if device == "" {
			return "", fmt.Errorf("Volume %s in pool %s not found in RBD showmapped output", do.Volume.Name, do.Volume.Params["pool"])
		}
	}

	log.Debugf("mapped volume %q as %q", do.Volume.Name, device)

	return device, nil
}

func (c *Driver) mkfsVolume(fscmd, devicePath string) error {
	// Create ext4 filesystem on the device. this will take a while
	if _, err := exec.Command("/bin/sh", "-c", templateFSCmd(fscmd, devicePath)).CombinedOutput(); err != nil {
		return fmt.Errorf("Error creating filesystem on %s with cmd: %q. Error: %v", devicePath, fscmd, err)
	}

	return nil
}

func (c *Driver) unmapImage(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	output, err := exec.Command("rbd", "showmapped", "--pool", poolName).Output()
	if err != nil {
		return err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return nil // no mapped images
	}

	// the first line is a header
	for _, line := range lines[1 : len(lines)-1] {
		parts := regexp.MustCompile(`\s+`).Split(line, -1)
		pool := parts[1]
		image := parts[2]
		device := parts[4]

		if strings.TrimSpace(pool) == poolName && strings.TrimSpace(image) == do.Volume.Name {
			log.Debugf("Unmapping volume %s/%s at device %q", poolName, do.Volume.Name, strings.TrimSpace(device))
			if err := exec.Command("rbd", "unmap", device).Run(); err != nil {
				return err
			}
		}
	}

	return c.unlockImage(do)
}
