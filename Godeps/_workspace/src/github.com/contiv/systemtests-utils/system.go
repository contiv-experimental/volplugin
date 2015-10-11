package utils

import (
	"time"

	log "github.com/Sirupsen/logrus"
)

// StopEtcd stops etcd on a specific host
func StopEtcd(node TestbedNode) error {
	log.Infof("Stopping etcd")
	return node.RunCommand("pkill etcd && rm -rf /tmp/etcd")
}

// StartEtcd starts etcd on a specific host.
func StartEtcd(node TestbedNode) error {
	log.Infof("Starting etcd")

	_, err := node.RunCommandBackground("nohup etcd -data-dir /tmp/etcd </dev/null &>/dev/null &")
	log.Infof("Waiting for etcd to finish starting")
	time.Sleep(10 * time.Millisecond)
	return err
}
