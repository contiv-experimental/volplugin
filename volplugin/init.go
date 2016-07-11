package volplugin

import (
	"strings"

	"golang.org/x/net/context"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage/backend"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"

	log "github.com/Sirupsen/logrus"
)

func (dc *DaemonConfig) updateMounts() error {
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return errored.Errorf("Could not initiate docker client").Combine(err)
	}

	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return errored.Errorf("Could not query docker").Combine(err)
	}

	for _, container := range containers {
		if container.State == "running" {
			for _, mount := range container.Mounts {
				if mount.Driver != "volplugin" {
					continue
				}
				dc.increaseMount(mount.Name)
			}
		}
	}

	for driverName := range backend.MountDrivers {
		cd, err := backend.NewMountDriver(driverName, dc.Global.MountPath)
		if err != nil {
			return err
		}

		mounts, err := cd.Mounted(dc.Global.Timeout)
		if err != nil {
			return err
		}

		for _, mount := range mounts {
			parts := strings.Split(mount.Volume.Name, "/")
			if len(parts) != 2 {
				log.Warnf("Invalid volume named %q in mount scan: skipping refresh", mount.Volume.Name)
				continue
			}

			log.Infof("Refreshing existing mount for %q", mount.Volume.Name)

			vol, err := dc.Client.GetVolume(parts[0], parts[1])
			if erd, ok := err.(*errored.Error); ok {
				switch {
				case erd.Contains(errors.NotExists):
					log.Warnf("Volume %q not found in database, skipping", mount.Volume.Name)
					continue
				case erd.Contains(errors.GetVolume):
					log.Fatalf("Volmaster could not be contacted; aborting volplugin.")
				}
			} else if err != nil {
				log.Fatalf("Unknown error reading from apiserver: %v", err)
			}

			// XXX some of the mounts get propagated above from docker itself, so
			// this is only necessary when that is missing reports.
			if dc.getMountCount(mount.Name) == 0 {
				dc.increaseMount(mount.Name)
			}

			payload := &config.UseMount{
				Volume:   mount.Volume.Name,
				Reason:   lock.ReasonMount,
				Hostname: dc.Host,
			}

			if vol.Unlocked {
				payload.Hostname = lock.Unlocked
			}

			stopChan, err := dc.Lock.AcquireWithTTLRefresh(payload, dc.Global.TTL, dc.Global.Timeout)
			if err != nil {
				log.Fatalf("Error encountered while trying to acquire lock for mount %v: %v", payload, err)
				continue
			}

			dc.addStopChan(mount.Volume.Name, stopChan)
			dc.addMount(mount)
		}
	}

	return nil
}
