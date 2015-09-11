package volcli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/config"
)

func errExit(ctx *cli.Context, err error) {
	fmt.Printf("\nError: %v\n\n", err)
	cli.ShowAppHelp(ctx)
	os.Exit(1)
}

// TenantUpload uploads a Tenant intent from stdin.
func TenantUpload(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, fmt.Errorf("Invalid arguments"))
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))

	content, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		errExit(ctx, err)
	}

	tenant := &config.TenantConfig{}

	if err := json.Unmarshal(content, tenant); err != nil {
		errExit(ctx, err)
	}

	if err := cfg.PublishTenant(ctx.Args()[0], tenant); err != nil {
		errExit(ctx, err)
	}
}

// TenantDelete removes a tenant supplied as an argument.
func TenantDelete(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, fmt.Errorf("Invalid arguments"))
	}

	tenant := ctx.Args()[0]

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err := cfg.DeleteTenant(tenant); err != nil {
		errExit(ctx, err)
	}

	fmt.Printf("%q removed!\n", tenant)
}
