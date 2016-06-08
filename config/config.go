package config

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
	"github.com/contiv/volplugin/watch"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

const (
	rootVolume    = "volumes"
	rootUse       = "users"
	rootPolicy    = "policies"
	rootSnapshots = "snapshots"
)

var defaultPaths = []string{rootVolume, rootUse, rootPolicy, rootSnapshots}

// Request provides a request structure for communicating with the
// volmaster.
type Request struct {
	Volume  string            `json:"volume"`
	Policy  string            `json:"policy"`
	Options map[string]string `json:"options"`
}

// Client is the top-level struct for communicating with the intent store.
type Client struct {
	etcdClient client.KeysAPI
	prefix     string
}

// NewClient creates a Client struct which can drive communication
// with the configuration store.
func NewClient(prefix string, etcdHosts []string) (*Client, error) {
	etcdCfg := client.Config{
		Endpoints: etcdHosts,
	}

	etcdClient, err := client.New(etcdCfg)
	if err != nil {
		return nil, err
	}

	config := &Client{
		prefix:     prefix,
		etcdClient: client.NewKeysAPI(etcdClient),
	}

	watch.Init(config.etcdClient)

	config.etcdClient.Set(context.Background(), config.prefix, "", &client.SetOptions{Dir: true})
	for _, path := range defaultPaths {
		config.etcdClient.Set(context.Background(), config.prefixed(path), "", &client.SetOptions{Dir: true})
	}

	return config, nil
}

func (c *Client) prefixed(strs ...string) string {
	str := c.prefix
	for _, s := range strs {
		str = path.Join(str, s)
	}

	return str
}

func addNodeToTarball(node *client.Node, writer *tar.Writer, baseDirectory string) error {
	now := time.Now()

	header := &tar.Header{
		AccessTime: now,
		ChangeTime: now,
		ModTime:    now,
		Name:       baseDirectory + node.Key,
	}

	if node.Dir {
		header.Mode = 0700
		header.Typeflag = tar.TypeDir
	} else {
		header.Mode = 0600
		header.Size = int64(len(node.Value))
		header.Typeflag = tar.TypeReg
	}

	err := writer.WriteHeader(header)
	if err != nil {
		return errored.Errorf("Failed to write tar entry header").Combine(err)
	}

	// we don't have to write anything for directories except the header
	if !node.Dir {
		_, err = writer.Write([]byte(node.Value))
		if err != nil {
			return errored.Errorf("Failed to write tar entry").Combine(err)
		}
	}

	for _, n := range node.Nodes {
		addNodeToTarball(n, writer, baseDirectory)
	}

	return nil
}

// DumpTarball dumps all the keys under the current etcd prefix into a
// gzip'd tarball'd directory-based representation of the namespace.
func (c *Client) DumpTarball() (string, error) {
	resp, err := c.etcdClient.Get(context.Background(), c.prefix, &client.GetOptions{Sort: true, Recursive: true, Quorum: true})
	if err != nil {
		return "", errored.Errorf(`Failed to recursively GET "%s" namespace from etcd`, c.prefix).Combine(errors.EtcdToErrored(err))
	}

	now := time.Now()

	// tar hangs during unpacking if the base directory has colons in it
	// unless --force-local is specified, so use the simpler "%Y%m%d-%H%M%S".
	niceTimeFormat := fmt.Sprintf("%d%02d%02d-%02d%02d%02d",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second())

	file, err := ioutil.TempFile("", "etcd_dump_"+niceTimeFormat+"_")
	if err != nil {
		return "", errored.Errorf("Failed to create tempfile").Combine(err)
	}
	defer file.Close()

	// create a gzipped, tarball writer which cleans up after itself
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// ensure that the tarball extracts to a folder with the same name as the tarball
	baseDirectory := filepath.Base(file.Name())

	err = addNodeToTarball(resp.Node, tarWriter, baseDirectory)
	if err != nil {
		return "", err
	}

	// give the file a more fitting name
	newFilename := file.Name() + ".tar.gz"

	err = os.Rename(file.Name(), newFilename)
	if err != nil {
		return "", err
	}

	return newFilename, nil
}
