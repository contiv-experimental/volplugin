package info

import (
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/volplugin/config"
)

func numFileDescriptors() int {
	fds, err := ioutil.ReadDir("/proc/self/fd")
	if err != nil {
		return -1
	}
	return len(fds)
}

func getCephVersion() (string, error) {
	cmd := exec.Command("ceph", "version")
	out, err := cmd.Output()
	if err != nil {
		return "", errored.Errorf("encountered error: %v", err)
	}

	output := strings.TrimLeft(string(out), "ceph version ")
	output = strings.TrimSpace(output)
	return output, nil
}

func logDebugInfo() {
	cephVersion, err := getCephVersion()
	if err != nil {
		cephVersion = "n/a"
	}

	log.WithFields(log.Fields{
		"file_descriptors": numFileDescriptors(),
		"goroutines":       runtime.NumGoroutine(),
		"architecture":     runtime.GOARCH,
		"os":               runtime.GOOS,
		"cpus":             runtime.NumCPU(),
		"go_version":       runtime.Version(),
		"ceph_version":     cephVersion,
	}).Info("received SIGUSR1; providing debug info")
}

// HandleDebugSignal watches for SIGUSR1 and logs the debug information
// using logrus
func HandleDebugSignal() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	for {
		select {
		case <-signals:
			logDebugInfo()
		}
	}
}

// HandleDumpTarballSignal watches for SIGUSR2 and creates a gzipped tarball
// of the current etcd directories/keys under the "/volplugin" namespace
func HandleDumpTarballSignal(client *config.Client) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR2)
	for {
		select {
		case <-signals:
			log.Info("received SIGUSR2; dumping etcd namespace to tarball")

			tarballPath, err := client.DumpTarball()
			if err != nil {
				log.Info("Failed to dump etcd namespace: ", err)
			} else {
				log.Info("Dumped etcd namespace to ", tarballPath)
			}
		}
	}
}
