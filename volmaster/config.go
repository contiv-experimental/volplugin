package main

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/contiv/go-etcd/etcd"
)

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

type config struct {
	etcdClient *etcd.Client
	prefix     string
	tenants    map[string]*configTenant
}

func newConfig(prefix string, etcdHosts []string) *config {
	return &config{
		prefix:     prefix,
		etcdClient: etcd.NewClient(etcdHosts),
		tenants:    map[string]*configTenant{},
	}
}

func (c *config) prefixed(str string) string {
	return path.Join(c.prefix, str)
}

func (c *config) validate() error {
	resp, err := c.etcdClient.Get(c.prefixed("tenants"), true, true)
	if err != nil {
		return err
	}

	for _, tenant := range resp.Node.Nodes {
		cfg := &configTenant{}
		if err := json.Unmarshal([]byte(tenant.Value), cfg); err != nil {
			return err
		}

		if cfg.Pool == "" {
			return fmt.Errorf("Config for tenant %q has a blank pool name", tenant.Key)
		}

		if cfg.Size == 0 {
			return fmt.Errorf("Config for tenant %q has a zero size", tenant.Key)
		}

		if cfg.UseSnapshots && (cfg.Snapshot.Frequency == "" || cfg.Snapshot.Keep == 0) {
			return fmt.Errorf("Snapshots are configured but cannot be used due to blank settings")
		}

		c.tenants[path.Base(tenant.Key)] = cfg
	}

	fmt.Println(c.tenants)

	return nil
}
