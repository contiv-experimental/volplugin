package main

import "fmt"

type configTenant struct {
	Pool         string         `json:"pool"`
	Size         uint64         `json:"size"`
	UseSnapshots bool           `json:"snapshots"`
	Snapshot     configSnapshot `json:"snapshot"`
}

type configSnapshot struct {
	Frequency string `json:"frequency"`
	Keep      uint   `json:"keep"`
}

type config map[string]configTenant

func (c config) validate() error {
	for tenant, cfg := range c {
		if cfg.Pool == "" {
			return fmt.Errorf("Config for tenant %q has a blank pool name", tenant)
		}

		if cfg.Size == 0 {
			return fmt.Errorf("Config for tenant %q has a zero size", tenant)
		}

		if cfg.UseSnapshots && (cfg.Snapshot.Frequency == "" || cfg.Snapshot.Keep == 0) {
			return fmt.Errorf("Snapshots are configured but cannot be used due to blank settings")
		}
	}

	return nil
}
