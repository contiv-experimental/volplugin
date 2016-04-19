package volcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/lock"
	"github.com/kr/pty"
)

func errorInvalidVolumeSyntax(rcvd, exptd string) error {
	return errored.Errorf("Invalid syntax: %q must be in the form of %q)", rcvd, exptd)
}

func errorInvalidArgCount(rcvd, exptd int, args []string) error {
	return errored.Errorf("Invalid number of arguments: expected %d but received %d %v", exptd, rcvd, args)
}

func splitVolume(ctx *cli.Context) (string, string, error) {
	volumeparts := strings.SplitN(ctx.Args()[0], "/", 2)

	if len(volumeparts) < 2 || volumeparts[0] == "" || volumeparts[1] == "" {
		return "", "", errorInvalidVolumeSyntax(ctx.Args()[0], `<policyName>/<volumeName>`)
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
	execCliAndExit(ctx, globalGet)
}

func queryGlobalConfig(ctx *cli.Context) (*config.Global, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/global", ctx.String("volmaster")))
	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errored.Errorf("Status code was %d not 200: %s", resp.StatusCode, string(content))
	}

	// rebuild and divide the contents so they are cast out of their internal
	// representation.
	return config.NewGlobalConfigFromJSON(content)
}

func globalGet(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 0 {
		return true, errorInvalidArgCount(len(ctx.Args()), 0, ctx.Args())
	}

	global, err := queryGlobalConfig(ctx)
	if err != nil {
		return false, err
	}

	content, err := ppJSON(global)
	if err != nil {
		return false, err
	}

	fmt.Println(string(content))
	return false, nil
}

// GlobalUpload uploads the global configuration
func GlobalUpload(ctx *cli.Context) {
	execCliAndExit(ctx, globalUpload)
}

func globalUpload(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 0 {
		return true, errorInvalidArgCount(len(ctx.Args()), 0, ctx.Args())
	}

	content, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return false, err
	}

	global := config.NewGlobalConfig()
	if err := json.Unmarshal(content, global); err != nil {
		return false, err
	}

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	if err := cfg.PublishGlobal(global); err != nil {
		return false, err
	}

	return false, nil
}

// PolicyUpload uploads a Policy intent from stdin.
func PolicyUpload(ctx *cli.Context) {
	execCliAndExit(ctx, policyUpload)
}

func policyUpload(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	content, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return false, err
	}

	policy := config.NewPolicy()

	if err := json.Unmarshal(content, policy); err != nil {
		return false, err
	}

	if err := cfg.PublishPolicy(ctx.Args()[0], policy); err != nil {
		return false, err
	}

	return false, nil
}

// PolicyDelete removes a policy supplied as an argument.
func PolicyDelete(ctx *cli.Context) {
	execCliAndExit(ctx, policyDelete)
}

func policyDelete(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	policy := ctx.Args()[0]

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	if err := cfg.DeletePolicy(policy); err != nil {
		return false, err
	}

	fmt.Printf("%q removed!\n", policy)

	return false, nil
}

// PolicyGet retrieves policy configuration, the name of which is supplied as
// an argument.
func PolicyGet(ctx *cli.Context) {
	execCliAndExit(ctx, policyGet)
}

func policyGet(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	policy := ctx.Args()[0]

	resp, err := http.Get(fmt.Sprintf("http://%s/policy/%s", ctx.String("volmaster"), policy))
	if err != nil {
		return false, err
	}

	if resp.StatusCode != 200 {
		return false, errored.Errorf("Response was not status 200: was %d: %v", resp.StatusCode, resp.Status)
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	fmt.Println(string(content))

	return false, nil
}

// PolicyList provides a list of the policy names.
func PolicyList(ctx *cli.Context) {
	execCliAndExit(ctx, policyList)
}

func policyList(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 0 {
		return true, errorInvalidArgCount(len(ctx.Args()), 0, ctx.Args())
	}

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	policies, err := cfg.ListPolicies()
	if err != nil {
		return false, err
	}

	for _, policy := range policies {
		fmt.Println(path.Base(policy))
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

	policy, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	opts := map[string]string{}

	for _, str := range ctx.StringSlice("opt") {
		pair := strings.SplitN(str, "=", 2)
		if len(pair) < 2 {
			return false, errored.Errorf("Mismatched option pair %q", pair)
		}

		opts[pair[0]] = pair[1]
	}

	tc := &config.RequestCreate{
		Policy: policy,
		Volume: volume,
		Opts:   opts,
	}

	content, err := json.Marshal(tc)
	if err != nil {
		return false, errored.Errorf("Could not create request JSON: %v", err)
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/create", ctx.String("volmaster")), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return false, errored.Errorf("Error in request: %v", err)
	}

	if resp.StatusCode != 200 {
		return false, errored.Errorf("Response Status Code was %d, not 200", resp.StatusCode)
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

	policy, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	vol, err := cfg.GetVolume(policy, volume)
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

	policy, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	if err := cfg.RemoveVolume(policy, volume); err != nil {
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

	policy, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	request := config.Request{
		Policy: policy,
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
		return false, errored.Errorf("Response Status Code was %d, not 200", resp.StatusCode)
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

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
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

// VolumeSnapshotTake takes a snapshot for a volume immediately.
func VolumeSnapshotTake(ctx *cli.Context) {
	execCliAndExit(ctx, volumeSnapshotTake)
}

func volumeSnapshotTake(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 3, ctx.Args())
	}

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	if err := cfg.TakeSnapshot(ctx.Args()[0]); err != nil {
		return false, err
	}

	return false, nil
}

// VolumeSnapshotCopy lists all snapshots for a given volume.
func VolumeSnapshotCopy(ctx *cli.Context) {
	execCliAndExit(ctx, volumeSnapshotCopy)
}

func volumeSnapshotCopy(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 3 {
		return true, errorInvalidArgCount(len(ctx.Args()), 3, ctx.Args())
	}

	policy, volume1, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	snapName := ctx.Args()[1]
	volume2 := ctx.Args()[2]

	req := &config.Request{
		Volume: volume1,
		Policy: policy,
		Options: map[string]string{
			"target":   volume2,
			"snapshot": snapName,
		},
	}

	content, err := json.Marshal(req)
	if err != nil {
		return false, errored.Errorf("Could not create request JSON: %v", err)
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/copy", ctx.String("volmaster")), "application/json", bytes.NewBuffer(content))
	if err != nil {
		return false, err
	}

	if resp.StatusCode != 200 {
		content, _ := ioutil.ReadAll(resp.Body)
		if content != nil {
			fmt.Println(string(content))
		}
		return false, errored.Errorf("Status was not 200: was %d: %v", resp.StatusCode, resp.Status)
	}

	return false, nil
}

// VolumeSnapshotList lists all snapshots for a given volume.
func VolumeSnapshotList(ctx *cli.Context) {
	execCliAndExit(ctx, volumeSnapshotList)
}

func volumeSnapshotList(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	policy, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/snapshots/%s/%s", ctx.String("volmaster"), policy, volume))
	if err != nil {
		return false, err
	}

	if resp.StatusCode != 200 {
		return false, errored.Errorf("Response was not status 200: was %d: %v", resp.StatusCode, resp.Status)
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var results []string

	if err := json.Unmarshal(content, &results); err != nil {
		return false, err
	}

	for _, result := range results {
		fmt.Println(result)
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

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
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

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	uses, err := cfg.ListUses("mount")
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

	policy, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	vc := &config.Volume{
		PolicyName: policy,
		VolumeName: volume,
	}

	mount := &config.UseMount{}

	if err := cfg.GetUse(mount, vc); err != nil {
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

	policy, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	vc := &config.Volume{
		PolicyName: policy,
		VolumeName: volume,
	}

	if err := cfg.RemoveUse(&config.UseMount{Volume: vc.String()}, true); err != nil {
		fmt.Fprintf(os.Stderr, "Trouble removing mount lock (may be harmless) for %q: %v", vc, err)
	}

	if err := cfg.RemoveUse(&config.UseSnapshot{Volume: vc.String()}, true); err != nil {
		fmt.Fprintf(os.Stderr, "Trouble removing snapshot lock (may be harmless) for %q: %v", vc, err)
	}

	return false, nil
}

// UseExec acquires a lock (waiting if necessary) and executes a command when it takes it.
func UseExec(ctx *cli.Context) {
	execCliAndExit(ctx, useExec)
}

func useExec(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) < 2 {
		return true, errorInvalidArgCount(len(ctx.Args()), 2, ctx.Args())
	}

	policy, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	vc := &config.Volume{
		PolicyName: policy,
		VolumeName: volume,
	}

	host, err := os.Hostname()
	if err != nil {
		return false, err
	}

	um := &config.UseMount{
		Volume:   vc.String(),
		Reason:   lock.ReasonMaintenance,
		Hostname: host,
	}

	us := &config.UseSnapshot{
		Volume: vc.String(),
		Reason: lock.ReasonMaintenance,
	}

	args := ctx.Args()[1:]
	if args[0] == "--" {
		if len(args) < 2 {
			return true, errored.Errorf("You must supply a command to run")
		}
		args = args[1:]
	}

	err = lock.NewDriver(cfg).ExecuteWithMultiUseLock([]config.UseLocker{um, us}, true, 0, func(ld *lock.Driver, uls []config.UseLocker) error {
		cmd := exec.Command("/bin/sh", "-c", strings.Join(args, " "))

		signals := make(chan os.Signal)

		go signal.Notify(signals, syscall.SIGINT)
		go func() {
			<-signals
			cmd.Process.Signal(syscall.SIGINT)
		}()

		if _, err := pty.Start(cmd); err != nil {
			return err
		}

		return cmd.Wait()
	})

	return false, err
}

// VolumeRuntimeGet retrieves the runtime configuration for a volume.
func VolumeRuntimeGet(ctx *cli.Context) {
	execCliAndExit(ctx, volumeRuntimeGet)
}

func volumeRuntimeGet(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	policy, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/runtime/%s/%s", ctx.String("volmaster"), policy, volume))
	if err != nil {
		return false, err
	}

	if resp.StatusCode != 200 {
		return false, errored.Errorf("Response was not status 200: was %d: %v", resp.StatusCode, resp.Status)
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	runtime := config.RuntimeOptions{}

	if err := json.Unmarshal(content, &runtime); err != nil {
		return false, err
	}

	content, err = ppJSON(runtime)
	if err != nil {
		return false, err
	}

	fmt.Println(string(content))

	return false, nil
}

// VolumeRuntimeUpload retrieves the runtime configuration for a volume.
func VolumeRuntimeUpload(ctx *cli.Context) {
	execCliAndExit(ctx, volumeRuntimeUpload)
}

func volumeRuntimeUpload(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 1 {
		return true, errorInvalidArgCount(len(ctx.Args()), 1, ctx.Args())
	}

	policy, volume, err := splitVolume(ctx)
	if err != nil {
		return true, err
	}

	cfg, err := config.NewClient(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))
	if err != nil {
		return false, err
	}

	content, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return false, err
	}

	runtime := config.RuntimeOptions{}

	if err := json.Unmarshal(content, &runtime); err != nil {
		return false, err
	}

	vol, err := cfg.GetVolume(policy, volume)
	if err != nil {
		return false, err
	}

	if err := cfg.PublishVolumeRuntime(vol, runtime); err != nil {
		return false, err
	}

	return false, nil
}
