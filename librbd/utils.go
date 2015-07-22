package librbd

// #include <errno.h>
// #include <string.h>
//
import "C"
import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func strerror(i C.int) error {
	return errors.New(C.GoString(C.strerror(-i)))
}

func (p *Pool) findDevice(imageName string) (string, error) {
	if name, err := p.findDeviceTree(imageName); err == nil {
		if _, err := os.Stat(rbdDev + name); err != nil {
			return "", err
		}

		return rbdDev + name, nil
	}

	return "", os.ErrNotExist
}

func modprobeRBD() error {
	return exec.Command("modprobe", "rbd").Run()
}

func (p *Pool) findDeviceTree(imageName string) (string, error) {
	fi, err := ioutil.ReadDir(rbdDevicePath)
	if err != nil && err != os.ErrNotExist {
		return "", err
	} else if err == os.ErrNotExist {
		return "", fmt.Errorf("Could not locate devices directory")
	}

	for _, f := range fi {
		namePath := filepath.Join(rbdDevicePath, f.Name(), "name")
		content, err := ioutil.ReadFile(namePath)
		if err != nil {
			return "", err
		}

		if strings.TrimSpace(string(content)) == imageName {
			poolPath := filepath.Join(rbdDevicePath, f.Name(), "pool")
			content, err := ioutil.ReadFile(poolPath)
			if err != nil {
				return "", err
			}

			if strings.TrimSpace(string(content)) == p.poolName {
				return f.Name(), err
			}
		}
	}

	return "", os.ErrNotExist
}
