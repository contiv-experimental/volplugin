package volcli

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/config"
)

// TenantUpload uploads a Tenant intent from stdin.
func TenantUpload(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, fmt.Errorf("Not enough arguments"))
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))

	content, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		errExit(ctx, err)
	}

	if err := cfg.PublishTenant(ctx.Args()[0], string(content)); err != nil {
		errExit(ctx, err)
	}
}

func errExit(ctx *cli.Context, err error) {
	fmt.Printf("\nError: %v\n\n", err)
	cli.ShowAppHelp(ctx)
	os.Exit(1)
}
