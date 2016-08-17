package systemtests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/errored"
	"github.com/contiv/remotessh"
	"github.com/contiv/volplugin/config"
)

var startedContainers struct {
	sync.Mutex
	names map[string]struct{}
}

var (
	volumeListMutex = sync.Mutex{}
	volumeList      = map[string]struct{}{}
)

func ClearEtcd(node remotessh.TestbedNode) {
	logrus.Infof("Clearing etcd data")
	node.RunCommand(`for i in $(etcdctl ls /); do etcdctl rm --recursive "$i"; done`)
}

// WaitForDone polls for checkDoneFn function to return true up until specified timeout
func WaitForDone(doneFn func() (string, bool), tickDur time.Duration, timeoutDur time.Duration, timeoutMsg string) (string, error) {
	tick := time.Tick(tickDur)
	timeout := time.After(timeoutDur)
	doneCount := 0
	for {
		select {
		case <-tick:
			if ctxt, done := doneFn(); done {
				doneCount++
				// add some resiliency to poll in order to avoid false positives,
				// while polling more frequently
				if doneCount == 2 {
					// end poll
					return ctxt, nil
				}
			}
			// continue polling
		case <-timeout:
			ctxt, done := doneFn()
			if !done {
				return ctxt, fmt.Errorf("wait timeout. Msg: %s", timeoutMsg)
			}
			return ctxt, nil
		}
	}
}

func volumeParts(volume string) (string, string) {
	parts := strings.SplitN(volume, "/", 2)
	return parts[0], parts[1] // panic, schmanick
}

func fqVolume(policy, volume string) string {
	return strings.Join([]string{policy, volume}, "/")
}

func genRandomVolume() string {
retry:
	volume := genRandomString("test", "", 20)
	volumeListMutex.Lock()
	if _, ok := volumeList[volume]; ok {
		volumeListMutex.Unlock()
		goto retry
	}
	volumeList[volume] = struct{}{}
	volumeListMutex.Unlock()

	return volume
}

func genRandomVolumes(count int) []string {
	res := []string{}

	for i := 0; i < count; i++ {
		res = append(res, genRandomVolume())
	}

	return res
}

// genRandomString returns a pseudo random string.
// It doesn't worry about name collisions much at the moment.
func genRandomString(prefix, suffix string, strlen int) string {
	charSet := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	randStr := make([]byte, 0, strlen)
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < strlen; i++ {
		randStr = append(randStr, charSet[rand.Int()%len(charSet)])
	}
	return prefix + string(randStr) + suffix
}

func (s *systemtestSuite) dockerRun(host string, tty, daemon bool, volume, command string) (string, error) {
	ttystr := ""
	daemonstr := ""

	if tty {
		ttystr = "-t"
	}

	if daemon {
		daemonstr = "-d"
	}

	// generate a container name.
	// The probability of name collisions in a test should be low as there are
	// about 62^10 possible strings. But we simply search for a container name
	// in `startedContainers` with some additional locking overhead to keep tests reliable.
	// Note: we don't remove the container name from the list on a run failure, this
	// allows full cleanup later.
	startedContainers.Lock()
	cName := genRandomString("", "", 10)
	for _, ok := startedContainers.names[cName]; ok; {
		cName = genRandomString("", "", 10)
	}
	startedContainers.names[cName] = struct{}{}
	startedContainers.Unlock()

	dockerCmd := fmt.Sprintf(
		"docker run --name %s -i %s %s -v %v:/mnt:nocopy alpine %s",
		cName,
		ttystr,
		daemonstr,
		volume,
		command,
	)

	logrus.Infof("Starting docker on %q with: %q", host, dockerCmd)

	str, err := s.vagrant.GetNode(host).RunCommandWithOutput(dockerCmd)
	if err != nil {
		return str, err
	}

	return str, nil
}

func (s *systemtestSuite) mon0cmd(command string) (string, error) {
	return s.vagrant.GetNode("mon0").RunCommandWithOutput(command)
}

func (s *systemtestSuite) volcli(command string) (string, error) {
	return s.mon0cmd("volcli " + command)
}

func (s *systemtestSuite) readIntent(fn string) (*config.Policy, error) {
	content, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	cfg := config.NewPolicy()

	if err := json.Unmarshal(content, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (s *systemtestSuite) purgeVolume(host, volume string) error {
	logrus.Infof("Purging %s on %s", volume, host)

	policy, name := volumeParts(volume)

	// ignore the error here so we get to the purge if we have to
	if out, err := s.vagrant.GetNode(host).RunCommandWithOutput(fmt.Sprintf("docker volume rm %s", volume)); err != nil {
		logrus.Error(out, err)
	}

	defer func() {
		if cephDriver() {
			s.vagrant.GetNode("mon0").RunCommand(fmt.Sprintf("sudo rbd snap purge rbd/%s.%s", policy, name))
			s.vagrant.GetNode("mon0").RunCommand(fmt.Sprintf("sudo rbd rm rbd/%s.%s", policy, name))
		}
	}()

	if out, err := s.volcli(fmt.Sprintf("volume remove %s", volume)); err != nil {
		logrus.Error(out)
		return err
	}

	return nil
}

func (s *systemtestSuite) createVolume(host, volume string, opts map[string]string) error {
	optsStr := []string{}
	policy, name := volumeParts(volume)

	if nfsDriver() {
		logrus.Infof("Making NFS mount directory /volplugin/%s/%s", policy, name)
		_, err := s.mon0cmd(fmt.Sprintf("sudo mkdir -p /volplugin/%s/%s && sudo chmod 4777 /volplugin/%s/%s", policy, name, policy, name))
		if err != nil {
			return err
		}

		if opts == nil {
			opts = map[string]string{}
		}

		mountstr := fmt.Sprintf("%s:/volplugin/%s/%s", s.mon0ip, policy, name)
		logrus.Infof("Mapping NFS mount %q", mountstr)
		opts["mount"] = mountstr
	}

	if opts != nil {
		for key, value := range opts {
			optsStr = append(optsStr, "--opt")
			optsStr = append(optsStr, key+"="+value)
		}
	}

	logrus.Infof("Creating %s on %q with options %q", volume, host, strings.Join(optsStr, " "))

	cmd := fmt.Sprintf("docker volume create -d volcontiv --name %s/%s %s", policy, name, strings.Join(optsStr, " "))

	if out, err := s.vagrant.GetNode(host).RunCommandWithOutput(cmd); err != nil {
		logrus.Info(string(out))
		return err
	}

	if out, err := s.volcli(fmt.Sprintf("volume get %s/%s", policy, name)); err != nil {
		logrus.Error(out)
		return err
	}

	return nil
}

func (s *systemtestSuite) uploadGlobal(configFile string) error {
	logrus.Infof("Uploading global configuration %s", configFile)
	out, err := s.volcli(fmt.Sprintf("global upload < /testdata/globals/%s.json", configFile))
	if err != nil {
		logrus.Error(out)
	}

	return err
}

func (s *systemtestSuite) clearNFS() {
	logrus.Info("Clearing NFS directories")
	s.mon0cmd("sudo rm -rf /volplugin && sudo mkdir /volplugin")
}

func (s *systemtestSuite) rebootstrap() error {
	s.clearContainers()
	stopVolsupervisor(s.vagrant.GetNode("mon0"))
	s.vagrant.IterateNodes(stopVolplugin)
	s.vagrant.IterateNodes(stopAPIServer)
	if cephDriver() {
		s.clearRBD()
	}

	if nfsDriver() {
		s.clearNFS()
	}

	logrus.Info("Clearing etcd")
	ClearEtcd(s.vagrant.GetNode("mon0"))

	if err := s.vagrant.IterateNodes(startAPIServer); err != nil {
		return err
	}

	if err := s.vagrant.IterateNodes(waitForAPIServer); err != nil {
		return err
	}

	if err := s.uploadGlobal("global1"); err != nil {
		return err
	}

	if err := startVolsupervisor(s.vagrant.GetNode("mon0")); err != nil {
		return err
	}

	if err := waitForVolsupervisor(s.vagrant.GetNode("mon0")); err != nil {
		return err
	}

	if err := s.vagrant.IterateNodes(startVolplugin); err != nil {
		return err
	}

	if err := s.vagrant.IterateNodes(waitForVolplugin); err != nil {
		return err
	}

	if out, err := s.uploadIntent("policy1", "policy1"); err != nil {
		logrus.Errorf("Intent upload failed. Error: %v, Output: %s", err, out)
		return err
	}

	return nil
}

func getDriver() string {
	driver := "ceph"
	if strings.TrimSpace(os.Getenv("USE_DRIVER")) != "" {
		driver = strings.TrimSpace(os.Getenv("USE_DRIVER"))
	}
	return driver
}

func cephDriver() bool {
	return getDriver() == "ceph"
}

func nullDriver() bool {
	return getDriver() == "null"
}

func nfsDriver() bool {
	return getDriver() == "nfs"
}

func (s *systemtestSuite) createExports() error {
	out, err := s.mon0cmd("sudo mkdir -p /volplugin")
	if err != nil {
		logrus.Error(out)
		return errored.Errorf("Creating volplugin root").Combine(err)
	}

	out, err = s.mon0cmd("echo /volplugin \\*\\(rw,no_root_squash\\) | sudo tee /etc/exports.d/basic.exports")
	if err != nil {
		logrus.Error(out)
		return errored.Errorf("Creating export").Combine(err)
	}

	out, err = s.mon0cmd("sudo exportfs -a")
	if err != nil {
		logrus.Error(out)
		return errored.Errorf("exportfs").Combine(err)
	}

	return nil
}

func (s *systemtestSuite) uploadIntent(policyName, fileName string) (string, error) {
	logrus.Infof("Uploading intent %q as policy %q", fileName, policyName)
	return s.volcli(fmt.Sprintf("policy upload %s < /testdata/%s/%s.json", policyName, getDriver(), fileName))
}

func runCommandUntilNoError(node remotessh.TestbedNode, cmd string, timeout int) error {
	runCmd := func() (string, bool) {
		if err := node.RunCommand(cmd); err != nil {
			return "", false
		}
		return "", true
	}
	timeoutMessage := fmt.Sprintf("timeout reached trying to run %v on %q", cmd, node.GetName())
	_, err := WaitForDone(runCmd, 10*time.Millisecond, time.Duration(timeout)*time.Second, timeoutMessage)
	return err
}

func waitForVolsupervisor(node remotessh.TestbedNode) error {
	logrus.Infof("Checking if volsupervisor is running on %q", node.GetName())
	err := runCommandUntilNoError(node, "docker inspect -f {{.State.Running}} volsupervisor | grep true", 30)
	if err == nil {
		logrus.Infof("Volsupervisor is running on %q", node.GetName())

	}
	return nil
}

func waitForAPIServer(node remotessh.TestbedNode) error {
	logrus.Infof("Checking if apiserver is running on %q", node.GetName())
	err := runCommandUntilNoError(node, "docker inspect -f {{.State.Running}} apiserver | grep true", 30)
	if err == nil {
		logrus.Infof("APIServer is running on %q", node.GetName())
	}

	then := time.Now()
	err = runCommandUntilNoError(node, "connwait 127.0.0.1:9005", 60)
	if err != nil {
		return err
	}
	logrus.Infof("Took %s for apiserver on %q to be accessible", time.Since(then), node.GetName())

	return nil
}

func waitForVolplugin(node remotessh.TestbedNode) error {
	logrus.Infof("Checking if volplugin is running on %q", node.GetName())
	err := runCommandUntilNoError(node, "docker inspect -f {{.State.Running}} volplugin | grep true", 30)
	if err == nil {
		logrus.Infof("Volplugin is running on %q", node.GetName())

	}
	return nil
}

func (s *systemtestSuite) pullDebian() error {
	logrus.Infof("Pulling alpine:latest on all boxes")
	return s.vagrant.SSHExecAllNodes("docker pull alpine")
}

func restartNetplugin(node remotessh.TestbedNode) error {
	logrus.Infof("Restarting netplugin on %q", node.GetName())
	err := node.RunCommand("sudo systemctl restart netplugin netmaster")
	if err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	return nil
}

func startVolsupervisor(node remotessh.TestbedNode) error {
	logrus.Infof("Starting the volsupervisor on %q", node.GetName())
	return node.RunCommandBackground("sudo systemctl start volsupervisor")
}

func stopVolsupervisor(node remotessh.TestbedNode) error {
	logrus.Infof("Stopping the volsupervisor on %q", node.GetName())
	defer time.Sleep(time.Second)
	return node.RunCommand("sudo systemctl stop volsupervisor")
}

func startAPIServer(node remotessh.TestbedNode) error {
	logrus.Infof("Starting the apiserver on %q", node.GetName())
	err := node.RunCommandBackground("sudo systemctl start apiserver")
	logrus.Infof("Waiting for apiserver startup on %q", node.GetName())
	return err
}

func stopAPIServer(node remotessh.TestbedNode) error {
	logrus.Infof("Stopping the apiserver on %q", node.GetName())
	defer time.Sleep(time.Second)
	return node.RunCommand("sudo systemctl stop apiserver")
}

func startVolplugin(node remotessh.TestbedNode) error {
	logrus.Infof("Starting the volplugin on %q", node.GetName())
	return node.RunCommandBackground("sudo systemctl start volplugin")
}

func stopVolplugin(node remotessh.TestbedNode) error {
	logrus.Infof("Stopping the volplugin on %q", node.GetName())
	defer time.Sleep(time.Second)
	return node.RunCommand("sudo systemctl stop volplugin")
}

func waitDockerizedServicesHost(node remotessh.TestbedNode) error {
	services := map[string]string{
		"etcd": "etcdctl cluster-health",
	}

	for s, cmd := range services {
		logrus.Infof("Waiting for %s on %q", s, node.GetName())
		out, err := WaitForDone(
			func() (string, bool) {
				out, err := node.RunCommandWithOutput(cmd)
				if err != nil {
					return out, false
				}
				return out, true
			}, 2*time.Second, time.Minute, fmt.Sprintf("service %s is not healthy", s))
		if err != nil {
			logrus.Infof("a dockerized service failed. Output: %s, Error: %v", out, err)
			return err
		}
	}
	return nil
}

func (s *systemtestSuite) waitDockerizedServices() error {
	return s.vagrant.IterateNodes(waitDockerizedServicesHost)
}

func restartDockerHost(node remotessh.TestbedNode) error {
	logrus.Infof("Restarting docker on %q", node.GetName())
	// note that for all these restart tasks we error out quietly to avoid other
	// hosts being cleaned up
	node.RunCommand("sudo service docker restart")
	return nil
}

func (s *systemtestSuite) restartDocker() error {
	return s.vagrant.IterateNodes(restartDockerHost)
}

func (s *systemtestSuite) restartNetplugin() error {
	return s.vagrant.IterateNodes(restartNetplugin)
}

func (s *systemtestSuite) clearContainerHost(node remotessh.TestbedNode) error {
	startedContainers.Lock()
	names := []string{}
	for name := range startedContainers.names {
		names = append(names, name)
	}
	startedContainers.Unlock()
	logrus.Infof("Clearing containers %v on %q", names, node.GetName())
	node.RunCommand(fmt.Sprintf("docker rm -f %s", strings.Join(names, " ")))
	return nil
}

func (s *systemtestSuite) clearContainers() error {
	logrus.Infof("Clearing containers")
	defer func() {
		startedContainers.Lock()
		startedContainers.names = map[string]struct{}{}
		startedContainers.Unlock()
	}()
	return s.vagrant.IterateNodes(s.clearContainerHost)
}

func (s *systemtestSuite) clearVolumeHost(node remotessh.TestbedNode) error {
	logrus.Infof("Clearing volumes on %q", node.GetName())
	node.RunCommand("docker volume ls | tail -n +2 | awk '{ print $2 }' | xargs docker volume rm")
	return nil
}

func (s *systemtestSuite) clearVolumes() error {
	return s.vagrant.IterateNodes(s.clearVolumeHost)
}

func (s *systemtestSuite) clearRBD() error {
	if !cephDriver() {
		return nil
	}

	logrus.Info("Clearing rbd images")

	s.vagrant.IterateNodes(func(node remotessh.TestbedNode) error {
		s.vagrant.GetNode(node.GetName()).RunCommandWithOutput("for img in $(sudo rbd showmapped | tail -n +2 | awk \"{ print \\$5 }\"); do sudo umount $img; sudo umount -f $img; done")
		return nil
	})

	s.vagrant.IterateNodes(func(node remotessh.TestbedNode) error {
		s.vagrant.GetNode(node.GetName()).RunCommandWithOutput("for img in $(sudo rbd showmapped | tail -n +2 | awk \"{ print \\$5 }\"); do sudo umount $img; sudo rbd unmap $img; done")
		return nil
	})

	out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput("for img in $(sudo rbd ls); do sudo rbd snap purge $img; sudo rbd rm $img; done")
	if err != nil {
		logrus.Info(out)
	}

	return err
}
