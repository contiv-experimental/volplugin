package main

import "fmt"

type configTenant struct {
	Pool string `json:"pool"`
	Size uint64 `json:"size"`
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
	}

	return nil
}
