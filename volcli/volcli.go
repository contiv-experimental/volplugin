package volcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/config"
)

func errExit(ctx *cli.Context, err error, help bool) {
	fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)
	if help {
		cli.ShowAppHelp(ctx)
	}
	os.Exit(1)
}

func ppJSON(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// TenantUpload uploads a Tenant intent from stdin.
func TenantUpload(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))

	content, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		errExit(ctx, err, false)
	}

	tenant := &config.TenantConfig{}

	if err := json.Unmarshal(content, tenant); err != nil {
		errExit(ctx, err, false)
	}

	if err := cfg.PublishTenant(ctx.Args()[0], tenant); err != nil {
		errExit(ctx, err, false)
	}
}

// TenantDelete removes a tenant supplied as an argument.
func TenantDelete(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	tenant := ctx.Args()[0]

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err := cfg.DeleteTenant(tenant); err != nil {
		errExit(ctx, err, false)
	}

	fmt.Printf("%q removed!\n", tenant)
}

// TenantGet retrieves tenant configuration, the name of which is supplied as
// an argument.
func TenantGet(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	tenant := ctx.Args()[0]

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	value, err := cfg.GetTenant(tenant)
	if err != nil {
		errExit(ctx, err, false)
	}

	content, err := ppJSON(value)
	if err != nil {
		errExit(ctx, err, false)
	}

	fmt.Println(string(content))
}

// TenantList provides a list of the tenant names.
func TenantList(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	tenants, err := cfg.ListTenants()
	if err != nil {
		errExit(ctx, err, false)
	}

	for _, tenant := range tenants {
		fmt.Println(tenant)
	}
}

// VolumeCreate creates a new volume with a JSON specification to store its
// information.
func VolumeCreate(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	opts := map[string]string{}

	for _, str := range ctx.StringSlice("opt") {
		pair := strings.SplitN(str, "=", 2)
		if len(pair) < 2 {
			errExit(ctx, fmt.Errorf("Mismatched option pair %q", pair), false)
		}

		opts[pair[0]] = pair[1]
	}

	tc := &config.RequestCreate{
		Tenant: ctx.Args()[0],
		Volume: ctx.Args()[1],
		Opts:   opts,
	}

	content, err := json.Marshal(tc)
	if err != nil {
		errExit(ctx, fmt.Errorf("Could not create request JSON: %v", err), false)
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/create", ctx.String("master")), "application/json", bytes.NewBuffer(content))
	if err != nil {
		errExit(ctx, err, false)
	}

	if resp.StatusCode != 200 {
		errExit(ctx, fmt.Errorf("Response Status Code was %d, not 200", resp.StatusCode), false)
	}
}

// VolumeGet retrieves the metadata for a volume and prints it.
func VolumeGet(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	vol, err := cfg.GetVolume(ctx.Args()[0], ctx.Args()[1])
	if err != nil {
		errExit(ctx, err, false)
	}

	content, err := ppJSON(vol)
	if err != nil {
		errExit(ctx, err, false)
	}

	fmt.Println(string(content))
}

// VolumeForceRemove removes a volume forcefully.
func VolumeForceRemove(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err := cfg.RemoveVolume(ctx.Args()[0], ctx.Args()[1]); err != nil {
		errExit(ctx, err, false)
	}
}

// VolumeRemove removes a volume, deleting the image beneath it.
func VolumeRemove(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	request := config.Request{
		Tenant: ctx.Args()[0],
		Volume: ctx.Args()[1],
	}

	content, err := json.Marshal(request)
	if err != nil {
		errExit(ctx, err, false)
	}

	if _, err := http.Post(fmt.Sprintf("http://%s/remove", ctx.String("master")), "application/json", bytes.NewBuffer(content)); err != nil {
		errExit(ctx, err, false)
	}
}

// VolumeList prints the list of volumes for a pool.
func VolumeList(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	vols, err := cfg.ListVolumes(ctx.Args()[0])
	if err != nil {
		errExit(ctx, err, false)
	}

	for name := range vols {
		fmt.Println(name)
	}
}

// VolumeListAll returns a list of the pools the volmaster knows about.
func VolumeListAll(ctx *cli.Context) {
	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	pools, err := cfg.ListAllVolumes()
	if err != nil {
		errExit(ctx, err, false)
	}

	for _, name := range pools {
		fmt.Println(name)
	}
}

// MountList returns a list of the mounts the volmaster knows about.
func MountList(ctx *cli.Context) {
	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	mounts, err := cfg.ListMounts()
	if err != nil {
		errExit(ctx, err, false)
	}

	for _, name := range mounts {
		fmt.Println(name)
	}
}

// MountGet retrieves the JSON information for a mount.
func MountGet(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	mount, err := cfg.GetMount(ctx.Args()[0], ctx.Args()[1])
	if err != nil {
		errExit(ctx, err, false)
	}

	content, err := ppJSON(mount)
	if err != nil {
		errExit(ctx, err, false)
	}

	fmt.Println(string(content))
}

// MountForceRemove deletes the mount entry from etcd; useful for clearing a
// stale mount.
func MountForceRemove(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err := cfg.RemoveMount(&config.MountConfig{Pool: ctx.Args()[0], Volume: ctx.Args()[1]}); err != nil {
		errExit(ctx, err, false)
	}
}
