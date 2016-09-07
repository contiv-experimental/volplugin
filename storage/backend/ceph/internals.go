package ceph

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/sys/unix"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/executor"
	"github.com/contiv/volplugin/storage"
)

type rbdMap map[string]struct {
	Pool   string `json:"pool"`
	Name   string `json:"name"`
	Device string `json:"device"`
}

func (c *Driver) mapImage(do storage.DriverOptions) (string, error) {
	var poolName string
	if err := do.Volume.Params.Get("pool", &poolName); err != nil {
		return "", err
	}

	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return "", err
	}

	retries := 0

retry:
	cmd := exec.Command("rbd", "map", intName, "--pool", poolName)
	er, err := runWithTimeout(cmd, do.Timeout)
	if retries < 10 && err != nil {
		logrus.Errorf("Error mapping image: %v (%v) (%v). Retrying.", intName, er, err)
		retries++
		goto retry
	}

	if err != nil || er.ExitStatus != 0 {
		return "", errored.Errorf("Could not map %q: %v (%v) (%v)", intName, er, err, er.Stderr)
	}

	var device string

	rbdmap, err := c.showMapped(do.Timeout)
	if err != nil {
		return "", err
	}

	for _, rbd := range rbdmap {
		if rbd.Name == intName && rbd.Pool == poolName {
			device = rbd.Device
			break
		}
	}

	if device == "" {
		return "", errored.Errorf("Volume %s in pool %s not found in RBD showmapped output", intName, poolName)
	}

	logrus.Debugf("mapped volume %q as %q", intName, device)

	return device, nil
}

func (c *Driver) mkfsVolume(fscmd, devicePath string, timeout time.Duration) error {
	cmd := exec.Command("/bin/sh", "-c", templateFSCmd(fscmd, devicePath))
	er, err := runWithTimeout(cmd, timeout)
	if err != nil || er.ExitStatus != 0 {
		return errored.Errorf("Error creating filesystem on %s with cmd: %q. Error: %v (%v) (%v) (%v)", devicePath, fscmd, er, err, strings.TrimSpace(er.Stdout), strings.TrimSpace(er.Stderr))
	}

	return nil
}

func (c *Driver) unmapImage(do storage.DriverOptions) error {
	rbdmap, err := c.showMapped(do.Timeout)
	if err != nil {
		return err
	}

	retry := true

	for retries := 0; retry && retries < 10; retries++ {
		var err error
		retry, err = c.doUnmap(do, rbdmap)
		if err != nil {
			return err
		}
	}

	if retry {
		return errored.Errorf("Could not unmap volume %q after 10 retries", do.Volume.Name)
	}

	return nil
}

func (c *Driver) doUnmap(do storage.DriverOptions, rbdmap rbdMap) (bool, error) {
	var poolName string
	if err := do.Volume.Params.Get("pool", &poolName); err != nil {
		return false, err
	}

	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return false, err
	}

	for _, rbd := range rbdmap {
		if rbd.Name == intName && rbd.Pool == poolName {
			logrus.Debugf("Unmapping volume %s/%s at device %q", poolName, intName, strings.TrimSpace(rbd.Device))

			if _, err := os.Stat(rbd.Device); err != nil {
				logrus.Debugf("Trying to unmap device %q for %s/%s that does not exist, continuing", poolName, intName, rbd.Device)
				continue
			}

			cmd := exec.Command("rbd", "unmap", rbd.Device)
			er, err := runWithTimeout(cmd, do.Timeout)
			if err != nil || er.ExitStatus != 0 {
				logrus.Errorf("Could not unmap volume %q (device %q): %v (%v) (%v)", intName, rbd.Device, er, err, er.Stderr)
				if er.ExitStatus == int(unix.EBUSY) {
					logrus.Errorf("Retrying to unmap volume %q (device %q)...", intName, rbd.Device)
					time.Sleep(100 * time.Millisecond)
					return true, nil
				}
				return false, err
			}

			rbdmap2, err := c.showMapped(do.Timeout)
			if err != nil {
				return false, err
			}

			for _, rbd2 := range rbdmap2 {
				if rbd.Name == rbd2.Name && rbd.Pool == rbd2.Pool {
					return true, nil
				}
			}

			break
		}
	}

	return false, nil
}

func (c *Driver) showMapped(timeout time.Duration) (rbdMap, error) {
	var (
		er  *executor.ExecResult
		err error
	)

retry:
	rbdmap := rbdMap{}

	cmd := exec.Command("rbd", "showmapped", "--format", "json")
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	er, err = executor.NewCapture(cmd).Run(ctx)
	if err != nil || er.ExitStatus != 0 || er.Stdout == "" {
		logrus.Warnf("Could not show mapped volumes. Retrying: %v", er.Stderr)
		time.Sleep(100 * time.Millisecond)
		goto retry
	}

	if err := json.Unmarshal([]byte(er.Stdout), &rbdmap); err != nil {
		logrus.Errorf("Could not parse RBD showmapped output, retrying: %s", er.Stderr)
		time.Sleep(100 * time.Millisecond)
		goto retry
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
				Name: c.externalName(rbd.Name),
				Params: storage.DriverParams{
					"pool": rbd.Pool,
				},
			},
		})
	}

	return mounts, nil
}
