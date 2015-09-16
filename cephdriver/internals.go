package cephdriver

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
)

func (cv *CephVolume) volumeCreate() error {
	return exec.Command("rbd", "create", cv.VolumeName, "--size", strconv.FormatUint(cv.VolumeSize, 10), "--pool", cv.PoolName).Run()
}

func (cv *CephVolume) mapImage() (string, error) {
	blkdev, err := exec.Command("rbd", "map", cv.VolumeName, "--pool", cv.PoolName).Output()
	device := strings.TrimSpace(string(blkdev))

	if err == nil {
		log.Debugf("mapped volume %q as %q", cv.VolumeName, device)
	}

	return device, err
}

func (cd *CephDriver) mkfsVolume(devicePath string) error {
	// Create ext4 filesystem on the device. this will take a while
	out, err := exec.Command("mkfs.ext4", "-m0", devicePath).CombinedOutput()

	if err != nil {
		log.Debug(string(out))
		return fmt.Errorf("Error creating ext4 filesystem on %s. Error: %v", devicePath, err)
	}

	return nil
}

func (cv *CephVolume) unmapImage() error {
	output, err := exec.Command("rbd", "showmapped", "--pool", cv.PoolName).Output()
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

		if strings.TrimSpace(pool) == cv.PoolName && strings.TrimSpace(image) == cv.VolumeName {
			log.Debugf("Unmapping volume %s/%s at device %q", cv.PoolName, cv.VolumeName, strings.TrimSpace(device))
			if err := exec.Command("rbd", "unmap", device).Run(); err != nil {
				return err
			}
		}
	}

	return nil
}
