package ceph

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/net/context"

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
	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("rbd", "map", intName, "--pool", poolName)
	er, err := runWithTimeout(cmd, do.Timeout)
	if err != nil || er.ExitStatus != 0 {
		return "", errored.Errorf("Could not map %q: %v (%v) (%v)", intName, er, err, er.Stderr)
	}

	var device string

	rbdmap, err := c.showMapped(do.Timeout)
	if err != nil {
		return "", err
	}

	for _, rbd := range rbdmap {
		if rbd.Name == intName && rbd.Pool == do.Volume.Params["pool"] {
			device = rbd.Device
			break
		}
	}

	if device == "" {
		return "", errored.Errorf("Volume %s in pool %s not found in RBD showmapped output", intName, do.Volume.Params["pool"])
	}

	log.Debugf("mapped volume %q as %q", intName, device)

	return device, nil
}

func (c *Driver) mkfsVolume(fscmd, devicePath string, timeout time.Duration) error {
	cmd := exec.Command("/bin/sh", "-c", templateFSCmd(fscmd, devicePath))
	er, err := runWithTimeout(cmd, timeout)
	if err != nil || er.ExitStatus != 0 {
		return errored.Errorf("Error creating filesystem on %s with cmd: %q. Error: %v (%v)", devicePath, fscmd, er, err)
	}

	return nil
}

func (c *Driver) unmapImage(do storage.DriverOptions) error {
	poolName := do.Volume.Params["pool"]

	intName, err := c.internalName(do.Volume.Name)
	if err != nil {
		return err
	}

	rbdmap, err := c.showMapped(do.Timeout)
	if err != nil {
		return err
	}

	var retried bool
retry:
	for _, rbd := range rbdmap {
		if rbd.Name == intName && rbd.Pool == do.Volume.Params["pool"] {
			log.Debugf("Unmapping volume %s/%s at device %q", poolName, intName, strings.TrimSpace(rbd.Device))

			if _, err := os.Stat(rbd.Device); err != nil {
				log.Debugf("Trying to unmap device %q for %s/%s that does not exist, continuing", poolName, intName, rbd.Device)
				continue
			}

			cmd := exec.Command("rbd", "unmap", rbd.Device)
			er, err := runWithTimeout(cmd, do.Timeout)
			if !retried && (err != nil || er.ExitStatus != 0) {
				log.Errorf("Could not unmap volume %q (device %q): %v (%v) (%v)", intName, rbd.Device, er, err, er.Stderr)
				if er.ExitStatus == 16 {
					log.Errorf("Retrying to unmap volume %q (device %q)...", intName, rbd.Device)
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

retry:
	rbdmap := rbdMap{}

	cmd := exec.Command("rbd", "showmapped", "--format", "json")
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	er, err = executor.NewCapture(cmd).Run(ctx)
	if err != nil || er.ExitStatus == 12 || er.Stdout == "" {
		log.Warnf("Could not show mapped volumes. Retrying: %v", er.Stderr)
		time.Sleep(100 * time.Millisecond)
		goto retry
	}

	if err := json.Unmarshal([]byte(er.Stdout), &rbdmap); err != nil {
		log.Errorf("Could not parse RBD showmapped output, retrying: %s", er.Stderr)
		time.Sleep(100 * time.Second)
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
				Params: map[string]string{
					"pool": rbd.Pool,
				},
			},
		})
	}

	return mounts, nil
}
