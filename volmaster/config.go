package main

import "fmt"

type configTenant struct {
	pool string
	size uint64
}

type Config map[string]configTenant

func (c Config) validate() error {
	for tenant, cfg := range c {
		if cfg.pool == "" {
			return fmt.Errorf("Config for tenant %q has a blank pool name", tenant)
		}

		if cfg.size == 0 {
			return fmt.Errorf("Config for tenant %q has a zero size", tenant)
		}
	}

	return nil
}
