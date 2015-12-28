package ceph

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"encoding/json"

	"github.com/contiv/volplugin/storage"

	log "github.com/Sirupsen/logrus"
)

func (c *Driver) mapImage(do storage.DriverOptions) (string, error) {
	var rbdMaps map[string] struct {
		Pool string `json:"pool"`
		Name string `json:"name"`
		Device string `json:"device"`
	}

	var device string

	blkdev, err := exec.Command("rbd", "map", do.Volume.Name, "--pool", do.Volume.Params["pool"]).Output()
	device = strings.TrimSpace(string(blkdev))

	if device == "" && err == nil {
		output, err := exec.Command("rbd", "showmapped", "--format", "json").Output()

		if err == nil {
			err := json.Unmarshal(output, &rbdMaps)
			if err == nil {
				for i := range rbdMaps {
					if rbdMaps[i].Name == do.Volume.Name && rbdMaps[i].Pool == do.Volume.Params["pool"] {
						device = rbdMaps[i].Device
						break
					}
				}
			} else {
				log.Debugf("Could not parse RBD showmapped output.")
			}
		}
	}

	if err == nil {
		log.Debugf("mapped volume %q as %q", do.Volume.Name, device)
	}

	return device, err
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

	return nil
}
