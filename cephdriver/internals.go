package cephdriver

import (
	"fmt"
	"os/exec"

	log "github.com/Sirupsen/logrus"
)

func (cv *CephVolume) volumeCreate() error {
	return cv.driver.pool.CreateImage(cv.VolumeName, cv.VolumeSize)
}

func (cv *CephVolume) mapImage() (string, error) {
	img, err := cv.driver.pool.GetImage(cv.VolumeName)
	if err != nil {
		return "", err
	}

	blkdev, err := img.MapDevice()
	log.Debugf("mapped volume %q as %q", cv.VolumeName, blkdev)

	return blkdev, err
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
	img, err := cv.driver.pool.GetImage(cv.VolumeName)
	if err != nil {
		return err
	}

	return img.UnmapDevice()
}
