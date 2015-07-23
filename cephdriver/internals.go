package cephdriver

import (
	"fmt"
	"os/exec"

	log "github.com/Sirupsen/logrus"
)

func (cd *CephDriver) volumeCreate(volumeName string, volumeSize uint64) error {
	return cd.pool.CreateImage(volumeName, volumeSize)
}

func (cd *CephDriver) mapImage(volumeName string) (string, error) {
	img, err := cd.pool.GetImage(volumeName)
	if err != nil {
		return "", err
	}

	blkdev, err := img.MapDevice()
	log.Debugf("mapped volume %q as %q", volumeName, blkdev)

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

func (cd *CephDriver) unmapImage(volumeName string) error {
	img, err := cd.pool.GetImage(volumeName)
	if err != nil {
		return err
	}

	return img.UnmapDevice()
}

func (cd *CephDriver) volumeExists(volumeName string) (bool, error) {
	list, err := cd.pool.List()
	if err != nil {
		return false, err
	}

	for _, volName := range list {
		if volName == volumeName {
			return true, nil
		}
	}

	return false, nil
}
