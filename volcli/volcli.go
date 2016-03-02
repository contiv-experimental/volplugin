package volcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/config"
)

func errorInvalidVolumeSyntax(rcvd, exptd string) error {
	return fmt.Errorf("Invalid syntax: %q must be in the form of %q)", rcvd, exptd)
}

func errorInvalidArgCount(rcvd, exptd int, args []string) error {
	return fmt.Errorf("Invalid number of arguments: expected %d but received %d %v", exptd, rcvd, args)
}

func splitVolume(ctx *cli.Context) (string, string, error) {
	volumeparts := strings.SplitN(ctx.Args()[0], "/", 2)

	if len(volumeparts) < 2 {
		return "", "", errorInvalidVolumeSyntax(ctx.Args()[0], `<tenantName>/<volumeName>`)
	}

	return volumeparts[0], volumeparts[1], nil
}

func errExit(ctx *cli.Context, err error, help bool) {
	fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)
	if help {
		cli.ShowAppHelp(ctx)
	}
	os.Exit(1)
}

func execCliAndExit(ctx *cli.Context, f func(ctx *cli.Context) (bool, error)) {
	if showHelp, err := f(ctx); err != nil {
		errExit(ctx, err, showHelp)
	}
}

func ppJSON(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// GlobalGet retrives the global configuration and displays it on standard output.
func GlobalGet(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/global", ctx.String("volmaster")))
	if err != nil {
		errExit(ctx, err, false)
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errExit(ctx, err, false)
	}

	if resp.StatusCode != 200 {
		errExit(ctx, fmt.Errorf("Status code was %d not 200: %s", resp.StatusCode, string(content)), false)
	}

	// rebuild and divide the contents so they are cast out of their internal
	// representation.
	global := &config.Global{}

	if err := json.Unmarshal(content, global); err != nil {
		errExit(ctx, err, false)
	}

	content, err = json.Marshal(config.DivideGlobalParameters(global))
	if err != nil {
		errExit(ctx, err, false)
	}

	fmt.Println(string(content))
}

// GlobalUpload uploads the global configuration
func GlobalUpload(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, fmt.Errorf("Invalid arguments"), true)
	}

	content, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		errExit(ctx, err, false)
	}

	global := &config.Global{}
	if err := json.Unmarshal(content, global); err != nil {
		errExit(ctx, err, false)
	}

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		errExit(ctx, err, false)
	}

	if err := cfg.PublishGlobal(global); err != nil {
		errExit(ctx, err, false)
	}
}

// TenantUpload uploads a Tenant intent from stdin.
func TenantUpload(ctx *cli.Context) {
	execCliAndExit(ctx, tenantUpload)
}

func tenantUpload(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	content, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return false, err
	}

	tenant := &config.TenantConfig{}

	if err := json.Unmarshal(content, tenant); err != nil {
		return false, err
	}

	if err := cfg.PublishTenant(ctx.Args()[0], tenant); err != nil {
		return false, err
	}

	return false, nil
}

// TenantDelete removes a tenant supplied as an argument.
func TenantDelete(ctx *cli.Context) {
	execCliAndExit(ctx, tenantDelete)
}

func tenantDelete(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	tenant := ctx.Args()[0]

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	if err := cfg.DeleteTenant(tenant); err != nil {
		return false, err
	}

	fmt.Printf("%q removed!\n", tenant)

	return false, nil
}

// TenantGet retrieves tenant configuration, the name of which is supplied as
// an argument.
func TenantGet(ctx *cli.Context) {
	execCliAndExit(ctx, tenantGet)
}

func tenantGet(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	tenant := ctx.Args()[0]

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	value, err := cfg.GetTenant(tenant)
	if err != nil {
		return false, err
	}

	content, err := ppJSON(value)
	if err != nil {
		return false, err
	}

	fmt.Println(string(content))

	return false, nil
}

// TenantList provides a list of the tenant names.
func TenantList(ctx *cli.Context) {
	execCliAndExit(ctx, tenantList)
}

func tenantList(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 0 {
		return true, errorInvalidArgCount(len(ctx.Args()), 0, ctx.Args())
	}

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	tenants, err := cfg.ListTenants()
	if err != nil {
		return false, err
	}

	for _, tenant := range tenants {
		fmt.Println(path.Base(tenant))
	}

	return false, nil
}

// VolumeCreate creates a new volume with a JSON specification to store its
// information.
func VolumeCreate(ctx *cli.Context) {
	execCliAndExit(ctx, volumeCreate)
}

func volumeCreate(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	tenant, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	opts := map[string]string{}

	for _, str := range ctx.StringSlice("opt") {
		pair := strings.SplitN(str, "=", 2)
		if len(pair) < 2 {
			return false, fmt.Errorf("Mismatched option pair %q", pair)
		}

		opts[pair[0]] = pair[1]
	}

	tc := &config.RequestCreate{
		Tenant: tenant,
		Volume: volume,
		Opts:   opts,
	}

	content, err := json.Marshal(tc)
	if err != nil {
		return false, fmt.Errorf("Could not create request JSON: %v", err)
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/create", ctx.String("volmaster")), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return false, err
	}

	if resp.StatusCode != 200 {
		return false, fmt.Errorf("Response Status Code was %d, not 200", resp.StatusCode)
	}

	return false, nil
}

// VolumeGet retrieves the metadata for a volume and prints it.
func VolumeGet(ctx *cli.Context) {
	execCliAndExit(ctx, volumeGet)
}

func volumeGet(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	tenant, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	vol, err := cfg.GetVolume(tenant, volume)
	if err != nil {
		return false, err
	}

	content, err := ppJSON(vol)
	if err != nil {
		return false, err
	}

	fmt.Println(string(content))

	return false, nil
}

// VolumeForceRemove removes a volume forcefully.
func VolumeForceRemove(ctx *cli.Context) {
	execCliAndExit(ctx, volumeForceRemove)
}

func volumeForceRemove(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	tenant, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	if err := cfg.RemoveVolume(tenant, volume); err != nil {
		return false, err
	}

	return false, nil
}

// VolumeRemove removes a volume, deleting the image beneath it.
func VolumeRemove(ctx *cli.Context) {
	execCliAndExit(ctx, volumeRemove)
}

func volumeRemove(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	tenant, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	request := config.Request{
		Tenant: tenant,
		Volume: volume,
	}

	content, err := json.Marshal(request)
	if err != nil {
		return false, err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/remove", ctx.String("volmaster")), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return false, err
	}

	if resp.StatusCode != 200 {
		io.Copy(os.Stderr, resp.Body)
		return false, fmt.Errorf("Response Status Code was %d, not 200", resp.StatusCode)
	}

	return false, nil
}

// VolumeList prints the list of volumes for a pool.
func VolumeList(ctx *cli.Context) {
	execCliAndExit(ctx, volumeList)
}

func volumeList(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	vols, err := cfg.ListVolumes(ctx.Args()[0])
	if err != nil {
		return false, err
	}

	for name := range vols {
		fmt.Println(name)
	}

	return false, nil
}

// VolumeListAll returns a list of the pools the volmaster knows about.
func VolumeListAll(ctx *cli.Context) {
	execCliAndExit(ctx, volumeListAll)
}

func volumeListAll(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 0 {
		return true, errorInvalidArgCount(len(ctx.Args()), 0, ctx.Args())
	}

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	pools, err := cfg.ListAllVolumes()
	if err != nil {
		return false, err
	}

	for _, name := range pools {
		fmt.Println(name)
	}

	return false, nil
}

// UseList returns a list of the mounts the volmaster knows about.
func UseList(ctx *cli.Context) {
	execCliAndExit(ctx, useList)
}

func useList(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 0 {
		return true, errorInvalidArgCount(len(ctx.Args()), 0, ctx.Args())
	}

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	uses, err := cfg.ListUses()
	if err != nil {
		return false, err
	}

	for _, name := range uses {
		fmt.Println(name)
	}

	return false, nil
}

// UseGet retrieves the JSON information for a mount.
func UseGet(ctx *cli.Context) {
	execCliAndExit(ctx, useGet)
}

func useGet(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	tenant, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	vc := &config.VolumeConfig{
		TenantName: tenant,
		VolumeName: volume,
	}

	mount, err := cfg.GetUse(vc)
	if err != nil {
		return false, err
	}

	content, err := ppJSON(mount)
	if err != nil {
		return false, err
	}

	fmt.Println(string(content))

	return false, nil
}

// UseTheForce deletes the use entry from etcd; useful for clearing a
// stale mount.
func UseTheForce(ctx *cli.Context) {
	execCliAndExit(ctx, useTheForce)
}

func useTheForce(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	tenant, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	cfg, err := config.NewTopLevelConfig(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	vc := &config.VolumeConfig{
		TenantName: tenant,
		VolumeName: volume,
	}

	if err := cfg.RemoveUse(&config.UseConfig{Volume: vc}, true); err != nil {
		return false, err
	}

	return false, nil
}
