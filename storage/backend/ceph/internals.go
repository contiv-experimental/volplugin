package ceph

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/contiv/errored"
	"github.com/contiv/executor"
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
		return "", errored.Errorf("Could not map %q: %v (%v) (%v)", do.Volume.Name, er, err, er.Stderr)
	}

	var device string

	rbdmap, err := c.showMapped(do.Timeout)
	if err != nil {
		return "", err
	}

	for _, rbd := range rbdmap {
		if rbd.Name == do.Volume.Name && rbd.Pool == do.Volume.Params["pool"] {
			device = rbd.Device
			break
		}
	}

	if device == "" {
		return "", errored.Errorf("Volume %s in pool %s not found in RBD showmapped output", do.Volume.Name, do.Volume.Params["pool"])
	}

	log.Debugf("mapped volume %q as %q", do.Volume.Name, device)

	return device, nil
}

func (c *Driver) mkfsVolume(fscmd, devicePath string, timeout time.Duration) error {
	cmd := exec.Command("/bin/sh", "-c", templateFSCmd(fscmd, devicePath))
	er, err := executor.NewWithTimeout(cmd, timeout).Run()
	if err != nil || er.ExitStatus != 0 {
		return errored.Errorf("Error creating filesystem on %s with cmd: %q. Error: %v (%v)", devicePath, fscmd, er, err)
	}

	return nil
}

func (c *Driver) unmapImage(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	rbdmap, err := c.showMapped(do.Timeout)
	if err != nil {
		return err
	}

	var retried bool
retry:
	for _, rbd := range rbdmap {
		if rbd.Name == do.Volume.Name && rbd.Pool == do.Volume.Params["pool"] {
			log.Debugf("Unmapping volume %s/%s at device %q", poolName, do.Volume.Name, strings.TrimSpace(rbd.Device))

			if _, err := os.Stat(rbd.Device); err != nil {
				log.Debugf("Trying to unmap device %q for %s/%s that does not exist, continuing", poolName, do.Volume.Name, rbd.Device)
				continue
			}

			er, err := executor.New(exec.Command("rbd", "unmap", rbd.Device)).Run()
			if !retried && (err != nil || er.ExitStatus != 0) {
				log.Errorf("Could not unmap volume %q (device %q): %v (%v) (%v)", do.Volume.Name, rbd.Device, er, err, er.Stderr)
				if er.ExitStatus == 4096 {
					log.Errorf("Retrying to unmap volume %q (device %q)...", do.Volume.Name, rbd.Device)
					time.Sleep(100 * time.Millisecond)
					retried = true
					goto retry
				}
				return err
			}

			if !retried {
				rbdmap2, err := c.showMapped(do.Timeout)
				if err != nil {
					return err
				}

				for _, rbd2 := range rbdmap2 {
					if rbd.Name == rbd2.Name && rbd.Pool == rbd2.Pool {
						retried = true
						goto retry
					}
				}
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
			continue
		} else {
			break
		}
	}

	rbdmap := rbdMap{}

	if err := json.Unmarshal([]byte(er.Stdout), &rbdmap); err != nil {
		return nil, errored.Errorf("Could not parse RBD showmapped output: %s", er.Stdout)
	}

	return rbdmap, nil
}

func (c *Driver) getMapped(timeout time.Duration) ([]*storage.Mount, error) {
	rbdmap, err := c.showMapped(timeout)
	if err != nil {
		return nil, err
	}

	mounts := []*storage.Mount{}

	for _, rbd := range rbdmap {
		mounts = append(mounts, &storage.Mount{
			Device: rbd.Device,
			Volume: storage.Volume{
				Name: strings.Replace(rbd.Name, ".", "/", -1),
				Params: map[string]string{
					"pool": rbd.Pool,
				},
			},
		})
	}

	return mounts, nil
}
