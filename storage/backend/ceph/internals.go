package ceph

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/contiv/volplugin/executor"
	"github.com/contiv/volplugin/storage"

	log "github.com/Sirupsen/logrus"
)

type rbdMap map[string]struct {
	Pool   string `json:"pool"`
	Name   string `json:"name"`
	Device string `json:"device"`
}

func (c *Driver) mapImage(do storage.DriverOptions) (string, error) {
	poolName := do.Volume.Params["pool"]

	cmd := exec.Command("rbd", "map", do.Volume.Name, "--pool", poolName)
	er, err := executor.NewWithTimeout(cmd, do.Timeout).Run()
	if err != nil || er.ExitStatus != 0 {
		return "", fmt.Errorf("Could not map %q: %v (%v)", do.Volume.Name, er, err)
	}

	var device string

	rbdmap, err := c.showMapped(do.Timeout)
	if err != nil {
		return "", err
	}

	for i := range rbdmap {
		if rbdmap[i].Name == do.Volume.Name && rbdmap[i].Pool == do.Volume.Params["pool"] {
			device = rbdmap[i].Device
			break
		}
	}

	if device == "" {
		return "", fmt.Errorf("Volume %s in pool %s not found in RBD showmapped output", do.Volume.Name, do.Volume.Params["pool"])
	}

	log.Debugf("mapped volume %q as %q", do.Volume.Name, device)

	return device, nil
}

func (c *Driver) mkfsVolume(fscmd, devicePath string, timeout time.Duration) error {
	// Create ext4 filesystem on the device. this will take a while
	cmd := exec.Command("/bin/sh", "-c", templateFSCmd(fscmd, devicePath))
	er, err := executor.NewWithTimeout(cmd, timeout).Run()
	if err != nil || er.ExitStatus != 0 {
		return fmt.Errorf("Error creating filesystem on %s with cmd: %q. Error: %v (%v)", devicePath, fscmd, er, err)
	}

	return nil
}

func (c *Driver) unmapImage(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	rbdmap, err := c.showMapped(do.Timeout)
	if err != nil {
		return err
	}

	for i := range rbdmap {
		if rbdmap[i].Name == do.Volume.Name && rbdmap[i].Pool == do.Volume.Params["pool"] {
			for {
				log.Debugf("Unmapping volume %s/%s at device %q", poolName, do.Volume.Name, strings.TrimSpace(rbdmap[i].Device))
				er, err := executor.New(exec.Command("rbd", "unmap", rbdmap[i].Device)).Run()
				if err != nil || er.ExitStatus != 0 {
					log.Errorf("Could not unmap volume %q (device %q): %v (%v)", do.Volume.Name, rbdmap[i].Device, er, err)
					if er.ExitStatus == 4096 {
						log.Errorf("Retrying to unmap volume %q (device %q)...", do.Volume.Name, rbdmap[i].Device)
						time.Sleep(100 * time.Millisecond)
						continue
					}
					return err
				}

				break
			}

			break
		}
	}

	return nil
}

func (c *Driver) showMapped(timeout time.Duration) (rbdMap, error) {
	var (
		er  *executor.ExecResult
		err error
	)

	for { // ugly
		cmd := exec.Command("rbd", "showmapped", "--format", "json")
		er, err = executor.NewWithTimeout(cmd, timeout).Run()
		if err != nil || er.ExitStatus == 3072 {
			log.Warnf("Could not show mapped volumes. Retrying")
			time.Sleep(100 * time.Millisecond)
		} else {
			break
		}
	}

	rbdmap := rbdMap{}

	if err := json.Unmarshal([]byte(er.Stdout), &rbdmap); err != nil {
		return nil, fmt.Errorf("Could not parse RBD showmapped output: %s", er.Stdout)
	}

	return rbdmap, nil
}

func (c *Driver) getMapped(timeout time.Duration) ([]*storage.Mount, error) {
	rbdmap, err := c.showMapped(timeout)
	if err != nil {
		return nil, err
	}

	mounts := []*storage.Mount{}

	for i := range rbdmap {
		mounts = append(mounts, &storage.Mount{
			Device: rbdmap[i].Device,
			Volume: storage.Volume{
				Name: strings.Replace(rbdmap[i].Name, ".", "/", -1),
				Params: map[string]string{
					"pool": rbdmap[i].Pool,
				},
			},
		})
	}

	return mounts, nil
}
