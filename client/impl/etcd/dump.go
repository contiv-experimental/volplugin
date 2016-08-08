package etcd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/net/context"

	"github.com/contiv/errored"
	etcd "github.com/coreos/etcd/client"
)

// Dump yields a database dump of the keyspace we manage. It will be contained
// in a tarball based on the timestamp of the dump. If a dir is provided, it
// will be placed under that directory.
func (e *Etcd) Dump(dir string) (string, error) {
	resp, err := e.client.Get(context.Background(), e.pather.String(), &etcd.GetOptions{Sort: true, Recursive: true, Quorum: true})
	if err != nil {
		return "", errored.Errorf(`Failed to recursively GET "%s" namespace from etcd`, e.pather).Combine(err)
	}

	// FIXME the code below (yes, all of it) should be moved somewhere else. It
	// doesn't belong in the etcd client.
	now := time.Now()

	// tar hangs during unpacking if the base directory has colons in it
	// unless --force-local is specified, so use the simpler "%Y%m%d-%H%M%S".
	niceTimeFormat := fmt.Sprintf("%d%02d%02d-%02d%02d%02d",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second())

	file, err := ioutil.TempFile(dir, "etcd_dump_"+niceTimeFormat+"_")
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

func addNodeToTarball(node *etcd.Node, writer *tar.Writer, baseDirectory string) error {
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
