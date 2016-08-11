package volmigrate

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/volmigrate/backend"
	"github.com/contiv/volplugin/volmigrate/backend/etcd2"
)

// -------------------------------------------------------------------------------------------------

// NOTE: These three functions are copied from volcli.go
//       Rather than creating a one-off package for them, they are intentionally directly copied.
//       If another app like volcli/volmigrate is added in the future, we should extract these
//       to a separate package.

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

func errorInvalidArgCount(rcvd, exptd int, args []string) error {
	return errored.Errorf("Invalid number of arguments: expected %d but received %d %v", exptd, rcvd, args)
}

// -------------------------------------------------------------------------------------------------

func promptBeforeRunning(ctx *cli.Context, msg string) {
	if ctx.GlobalBool("silent") {
		return
	}

	fmt.Println(msg)
	fmt.Println("Are you sure you want to continue? y/n")

	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read input from stdin: %s", err)
	}

	text = strings.ToLower(text)
	text = text[:len(text)-1] // strip newline

	if text != "yes" && text != "y" {
		fmt.Println("Aborting.")
		os.Exit(1)
	}
}

// ListMigrations returns a newline-delimited list of migration versions and their descriptions.
func ListMigrations(ctx *cli.Context) {
	execCliAndExit(ctx, listMigrations)
}

func listMigrations(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 0 {
		return true, errorInvalidArgCount(len(ctx.Args()), 0, ctx.Args())
	}

	for i := int64(1); i <= latestMigrationVersion; i++ {
		m := availableMigrations[i]

		fmt.Printf("%d - %s\n", m.Version, m.Description)
	}

	return false, nil
}

// RunMigrations runs either a single pending migration or all pending migrations.
// If no migrations need to be run, a message will state that nothing was performed.
func RunMigrations(ctx *cli.Context) {
	execCliAndExit(ctx, runMigrations)
}

func runMigrations(ctx *cli.Context) (bool, error) {
	e := etcd2.New(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))

	if len(ctx.Args()) > 0 {
		i, err := strconv.Atoi(ctx.Args()[0])
		if err != nil {
			return true, errored.Errorf("Invalid migration version: %v", ctx.Args()[0])
		}

		targetVersion := int64(i)

		if targetVersion < 1 || targetVersion > latestMigrationVersion {
			return false, errored.Errorf("%d is outside valid migration range - min: 1, max %d", targetVersion, latestMigrationVersion)
		}

		promptBeforeRunning(ctx, fmt.Sprintf("You have requested to run all pending migrations up to and including #%d.", targetVersion))
		return runMigrationsUpTo(e, targetVersion)
	}

	promptBeforeRunning(ctx, "You have requested to run all pending migrations.")
	return runAllMigrations(e)
}

func runAllMigrations(b backend.Backend) (bool, error) {
	return runMigrationsUpTo(b, latestMigrationVersion)
}

func runMigrationsUpTo(b backend.Backend, targetVersion int64) (bool, error) {
	migrationsRun := 0

	currentVersion := b.CurrentSchemaVersion()

	for i := int64(1); i <= latestMigrationVersion; i++ {
		m := availableMigrations[i]

		// skip migrations we've already run
		if currentVersion >= m.Version {
			continue
		}

		// run migrations up to and through our target version.
		if m.Version > targetVersion {
			break
		}

		if err := m.Run(b); err != nil {
			return false, err
		}

		migrationsRun++
	}

	if migrationsRun == 0 {
		fmt.Println("No migrations needed to be run.")
	} else {
		fmt.Println("All migrations ran successfully.")
	}

	return false, nil
}

// ShowVersions prints the current schema version and the newest version that's available.
func ShowVersions(ctx *cli.Context) {
	execCliAndExit(ctx, showVersions)
}

func showVersions(ctx *cli.Context) (bool, error) {
	if len(ctx.Args()) != 0 {
		return true, errorInvalidArgCount(len(ctx.Args()), 0, ctx.Args())
	}

	e := etcd2.New(ctx.GlobalString("prefix"), ctx.GlobalStringSlice("etcd"))

	fmt.Printf("Current local schema version: %d\n", e.CurrentSchemaVersion())
	fmt.Printf("Newest available schema version: %d\n", latestMigrationVersion)

	return false, nil
}
