package volplugin

import (
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/lock"
	"github.com/contiv/volplugin/storage"
	"github.com/contiv/volplugin/storage/backend"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"

	log "github.com/Sirupsen/logrus"
)

func (dc *DaemonConfig) getMounted() (map[string]*storage.Mount, map[string]int, error) {
	mounts := map[string]*storage.Mount{}
	counts := map[string]int{}

	now := time.Now()

	// XXX this loop will indefinitely run if the docker service is down.
	// This is intentional to ensure we don't take any action when docker is down.
	for {
		dockerClient, err := client.NewEnvClient()
		if err != nil {
			return nil, nil, errored.Errorf("Could not initiate docker client").Combine(err)
		}

		containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
		if err != nil {
			if now.Sub(time.Now()) > dc.Global.Timeout {
				panic("Cannot contact docker")
			}
			log.Error(errored.Errorf("Could not query docker; retrying").Combine(err))
			time.Sleep(time.Second)
			continue
		}

		for _, container := range containers {
			if container.State == "running" {
				for _, mount := range container.Mounts {
					if mount.Driver == "volplugin" {
						mounts[mount.Name] = nil
						counts[mount.Name]++
					}
				}
			}
		}

		break
	}

	for driverName := range backend.MountDrivers {
		cd, err := backend.NewMountDriver(driverName, dc.Global.MountPath)
		if err != nil {
			return nil, nil, err
		}

		mounted, err := cd.Mounted(dc.Global.Timeout)
		if err != nil {
			return nil, nil, err
		}

		for _, mount := range mounted {
			log.Debugf("Refreshing existing mount for %q: %v", mount.Volume.Name, *mount)
			mounts[mount.Volume.Name] = mount
		}
	}

	return mounts, counts, nil
}

func (dc *DaemonConfig) updateMounts() error {
	// this loop is used to ensure that volplugin itself has a clear understanding of what's going on.
	//
	// FIXME If volplugin detected no mount but docker thinks there is a volplugin
	// mount, we need to heal it somehow.

	mountNames, counts, err := dc.getMounted()
	if err != nil {
		return err
	}

	for name, mount := range mountNames {
		log.Debugf("%s: %#v", name, *mount)
		if mount != nil {
			dc.API.MountCounter.AddCount(name, counts[name])

			parts := strings.Split(name, "/")
			if len(parts) != 2 {
				log.Warnf("Invalid volume named %q in mount scan: skipping refresh", name)
				continue
			}

			vol, err := dc.Client.GetVolume(parts[0], parts[1])
			if erd, ok := err.(*errored.Error); ok {
				switch {
				case erd.Contains(errors.NotExists):
					log.Warnf("Volume %q not found in database, skipping", name)
					continue
				case erd.Contains(errors.GetVolume):
					log.Fatalf("Volmaster could not be contacted; aborting volplugin.")
				}
			} else if err != nil {
				log.Fatalf("Unknown error reading from apiserver: %v", err)
			}

			payload := &config.UseMount{
				Volume:   name,
				Reason:   lock.ReasonMount,
				Hostname: dc.API.Hostname,
			}

			if vol.Unlocked {
				payload.Hostname = lock.Unlocked
			}

			// only populate the mount if it doesn't already exist.
			if _, err := dc.API.MountCollection.Get(name); err != nil {
				dc.API.MountCollection.Add(mount)
				// since this may run twice, it will terminate the original goroutine via the original stop channel.
				stopChan, err := dc.API.Lock.AcquireWithTTLRefresh(payload, dc.Global.TTL, dc.Global.Timeout)
				if err != nil {
					log.Fatalf("Error encountered while trying to acquire lock for mount %v: %v", payload, err)
					continue
				}

				dc.API.AddStopChan(name, stopChan)
			}
		} else {
			log.Errorf("Missing mount data for %q which was reported by volplugin or docker as previously mounted", name)
		}
	}

	return nil
}
