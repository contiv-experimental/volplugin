package utils

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
)

// StopEtcd stops etcd on a specific host
func StopEtcd(nodes []TestbedNode) error {
	for _, node := range nodes {
		log.Infof("Stopping etcd")

		if err := node.RunCommand("sudo systemctl stop etcd; rm -rf /var/lib/etcd"); err != nil {
			return err
		}

		times := 10

		for {
			if err := node.RunCommand("etcdctl member list"); err != nil {
				break
			}

			times--

			if times < 0 {
				return fmt.Errorf("Timed out stopping etcd on %s", node.GetName())
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}

func ClearEtcd(node TestbedNode) {
	node.RunCommand("etcdctl ls --recursive / | xargs etcdctl rm --recursive")
}

// StartEtcd starts etcd on a specific host.
func StartEtcd(nodes []TestbedNode) error {
	for _, node := range nodes {
		log.Infof("Starting etcd on %s", node.GetName())
		times := 10

		for {
			// the error is not checked here because we will not successfully start
			// etcd the second time we try, but want to retry if the first one fails.
			node.RunCommand("sudo systemctl start etcd")

			time.Sleep(1 * time.Second)

			if err := node.RunCommand("etcdctl member list"); err == nil {
				break
			}

			times--

			if times < 0 {
				return fmt.Errorf("Timed out starting etcd on %s", node.GetName())
			}
		}
	}

	return nil
}
