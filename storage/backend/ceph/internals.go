package ceph

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/contiv/volplugin/executor"
	"github.com/contiv/volplugin/storage"

	log "github.com/Sirupsen/logrus"
)

func (c *Driver) lockImage(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	er, err := executor.New(exec.Command("rbd", "lock", "add", do.Volume.Name, do.Volume.Name, "--pool", poolName)).Run()
	if err != nil {
		return fmt.Errorf("Could not acquire lock for %q: %v", do.Volume.Name, err)
	}

	if er.ExitStatus != 0 {
		return fmt.Errorf("Could not acquire lock for %q: %v", do.Volume.Name, err)
	}

	return nil
}

func (c *Driver) unlockImage(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	er, err := executor.New(exec.Command("rbd", "lock", "--format", "json", "list", do.Volume.Name, "--pool", poolName)).Run()
	if err != nil {
		return fmt.Errorf("Error running `rbd lock list` for volume %q: %v", do.Volume.Name, err)
	}

	if er.ExitStatus != 0 {
		return fmt.Errorf("Error running `rbd lock list` for volume %q: %v", do.Volume.Name, er)
	}

	locks := map[string]map[string]string{}

	if err := json.Unmarshal([]byte(er.Stdout), &locks); err != nil {
		return fmt.Errorf("Error unmarshalling lock report for volume %q: %v", do.Volume.Name, err)
	}

	if _, ok := locks[do.Volume.Name]; ok {
		er, err := executor.New(exec.Command("rbd", "lock", "remove", do.Volume.Name, do.Volume.Name, locks[do.Volume.Name]["locker"], "--pool", poolName)).Run()
		if err != nil {
			return fmt.Errorf("Error releasing lock on volume %q: %v", do.Volume.Name, err)
		}

		if er.ExitStatus != 0 {
			return fmt.Errorf("Error releasing lock on volume %q: %v", do.Volume.Name, er)
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

	er, err := executor.New(exec.Command("rbd", "map", do.Volume.Name, "--pool", poolName)).Run()
	if err != nil {
		return "", fmt.Errorf("Could not map %q: %v", do.Volume.Name, err)
	}
	if er.ExitStatus != 0 {
		return "", fmt.Errorf("Could not map %q: %v", do.Volume.Name, er)
	}

	device := strings.TrimSpace(er.Stdout)

	if device == "" {
		er, err := executor.New(exec.Command("rbd", "showmapped", "--format", "json")).Run()
		if err != nil {
			return "", fmt.Errorf("Could not show mapped volumes: %v", err)
		}

		if er.ExitStatus != 0 {
			return "", fmt.Errorf("Could not show mapped volumes: %v", er)
		}

		if err := json.Unmarshal([]byte(er.Stdout), &rbdMaps); err != nil {
			return "", fmt.Errorf("Could not parse RBD showmapped output: %s", er.Stdout)
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
	er, err := executor.New(exec.Command("/bin/sh", "-c", templateFSCmd(fscmd, devicePath))).Run()
	if err != nil {
		return fmt.Errorf("Error creating filesystem on %s with cmd: %q. Error: %v", devicePath, fscmd, err)
	}
	if er.ExitStatus != 0 {
		return fmt.Errorf("Error creating filesystem on %s with cmd: %q. Error: %v", devicePath, fscmd, er)
	}

	return nil
}

func (c *Driver) unmapImage(do storage.DriverOptions) error {
	// FIXME use mapImage showmapped json code
	poolName := do.Volume.Params["pool"]

	er, err := executor.New(exec.Command("rbd", "showmapped", "--pool", poolName)).Run()
	if err != nil {
		return fmt.Errorf("Could not show mapped rbd volumes for pool %q: %v", poolName, err)
	}
	if er.ExitStatus != 0 {
		return fmt.Errorf("Could not show mapped rbd volumes for pool %q: %v", poolName, er)
	}

	lines := strings.Split(er.Stdout, "\n")
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
			er, err := executor.New(exec.Command("rbd", "unmap", device)).Run()
			if err != nil {
				return fmt.Errorf("Could not unmap volume %q (device %q): %v", do.Volume.Name, device, err)
			}
			if er.ExitStatus != 0 {
				return fmt.Errorf("Could not unmap volume %q (device %q): %v", do.Volume.Name, device, er)
			}
		}
	}

	return c.unlockImage(do)
}
