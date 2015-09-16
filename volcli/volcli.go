package volcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/config"
)

func errExit(ctx *cli.Context, err error) {
	fmt.Printf("\nError: %v\n\n", err)
	cli.ShowAppHelp(ctx)
	os.Exit(1)
}

func ppJSON(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
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

// TenantGet retrieves tenant configuration, the name of which is supplied as
// an argument.
func TenantGet(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, fmt.Errorf("Invalid arguments"))
	}

	tenant := ctx.Args()[0]

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	value, err := cfg.GetTenant(tenant)
	if err != nil {
		errExit(ctx, err)
	}

	// The following lines pretty-print the json by re-evaluating it. This is
	// purely a nicety for the CLI and is not necessary to use the tool.
	tenantObj := &config.TenantConfig{}

	if err := json.Unmarshal([]byte(value), tenantObj); err != nil {
		errExit(ctx, err)
	}

	content, err := ppJSON(tenantObj)
	if err != nil {
		errExit(ctx, err)
	}

	fmt.Println(string(content))
}

// TenantList provides a list of the tenant names.
func TenantList(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, fmt.Errorf("Invalid arguments"))
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	tenants, err := cfg.ListTenants()
	if err != nil {
		errExit(ctx, err)
	}

	for _, tenant := range tenants {
		fmt.Println(tenant)
	}
}

// VolumeGet retrieves the metadata for a volume and prints it.
func VolumeGet(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, fmt.Errorf("Invalid arguments"))
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	vol, err := cfg.GetVolume(ctx.Args()[0], ctx.Args()[1])
	if err != nil {
		errExit(ctx, err)
	}

	content, err := ppJSON(vol)
	if err != nil {
		errExit(ctx, err)
	}

	fmt.Println(string(content))
}

// VolumeForceRemove removes a volume forcefully.
func VolumeForceRemove(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, fmt.Errorf("Invalid arguments"))
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err := cfg.RemoveVolume(ctx.Args()[0], ctx.Args()[1]); err != nil {
		errExit(ctx, err)
	}
}

// VolumeRemove removes a volume, deleting the image beneath it.
func VolumeRemove(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, fmt.Errorf("Invalid arguments"))
	}

	request := config.Request{
		Pool:   ctx.Args()[0],
		Volume: ctx.Args()[1],
	}

	content, err := json.Marshal(request)
	if err != nil {
		errExit(ctx, err)
	}

	if _, err := http.Post(fmt.Sprintf("http://%s/remove", ctx.String("master")), "application/json", bytes.NewBuffer(content)); err != nil {
		errExit(ctx, err)
	}
}
