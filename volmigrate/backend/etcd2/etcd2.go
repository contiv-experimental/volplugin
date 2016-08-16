package etcd2

import (
	"fmt"
	"path"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/volmigrate/backend"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// New creates a new etcd2 migration engine.
func New(prefix string, etcdHosts []string) *Engine {
	etcdCfg := client.Config{
		Endpoints: etcdHosts,
	}

	etcdClient, err := client.New(etcdCfg)
	if err != nil {
		logrus.Fatalf("Failed to create etcd client: %s", err)
	}

	return &Engine{
		etcdClient: client.NewKeysAPI(etcdClient),
		prefix:     prefix,
	}
}

// Engine is a migration engine for an etcd v2 datastore.
type Engine struct {
	etcdClient client.KeysAPI
	prefix     string
}

// CurrentSchemaVersion returns the version of the last migration which was successfully run.
// If no previous migrations have been run, the schema version is 0.
// If there's any error besides "key doesn't exist", execution is aborted.
func (e *Engine) CurrentSchemaVersion() int64 {
	resp, err := e.etcdClient.Get(context.Background(), path.Join(e.prefix, backend.SchemaVersionKey), nil)
	if err != nil {
		if err := errors.EtcdToErrored(err); err != nil && err == errors.NotExists {
			return 0 // no key = schema version 0
		}

		logrus.Fatalf("Unexpected error when looking up schema version: %v\n", err)
	}

	i, err := strconv.Atoi(resp.Node.Value)
	if err != nil {
		logrus.Fatalf("Got back unexpected schema version data: %v\n", resp.Node.Value)
	}

	return int64(i)
}

// CreateDirectory creates a directory at the target path.
func (e *Engine) CreateDirectory(target string) error {
	fmt.Println("Creating directory: " + target)
	if _, err := e.etcdClient.Set(context.Background(), path.Join(e.prefix, target), "", &client.SetOptions{Dir: true}); err != nil {
		return errored.Errorf("Failed to create directory: %s", err)
	}

	return nil
}

// CreateKey will create a key at the target location.
func (e *Engine) CreateKey(target string, contents []byte) error {
	fmt.Println("Creating key: " + target)
	if _, err := e.etcdClient.Set(context.Background(), path.Join(e.prefix, target), string(contents), nil); err != nil {
		errored.Errorf("Failed to create key: %s", err)
	}

	return nil
}

// DeleteDirectory will recursively delete a target directory.
func (e *Engine) DeleteDirectory(target string) error {
	fmt.Println("Deleting directory: " + target)

	if _, err := e.etcdClient.Delete(context.Background(), path.Join(e.prefix, target), &client.DeleteOptions{Dir: true, Recursive: true}); err != nil {
		return errored.Errorf("Failed to delete directory: %s", err)
	}

	return nil
}

// DeleteKey will delete the target key if it exists.
func (e *Engine) DeleteKey(target string) error {
	fmt.Println("Deleting key: " + target)

	if _, err := e.etcdClient.Delete(context.Background(), path.Join(e.prefix, target), &client.DeleteOptions{Dir: false}); err != nil {
		return errored.Errorf("Failed to delete key: %s", err)
	}

	return nil
}

// Name returns the name of the engine ("etcd2", "etcd3", "consul", etc.)
func (e *Engine) Name() string {
	return "etcd2"
}

// UpdateSchemaVersion records the version number of the latest migration which successfully ran.
// If the new version number is <= the current version number, it will log a fatal error.
func (e *Engine) UpdateSchemaVersion(newVersion int64) error {
	fmt.Printf("Updating schema version key to: %d\n", newVersion)

	currentVersion := e.CurrentSchemaVersion()

	// sanity check to make sure the version number is actually increasing
	if newVersion <= currentVersion {
		logrus.Fatalf("Cowardly refusing to update schema version to a version <= the current version.  Current: %d, Desired: %d\n", currentVersion, newVersion)
	}

	data := strconv.Itoa(int(newVersion))

	if _, err := e.etcdClient.Set(context.Background(), path.Join(e.prefix, backend.SchemaVersionKey), data, nil); err != nil {
		return errored.Errorf("Failed to update schema version key to %d: %s", newVersion, err)
	}

	return nil
}
