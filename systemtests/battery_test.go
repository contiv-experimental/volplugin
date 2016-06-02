package systemtests

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/vagrantssh"
)

func (s *systemtestSuite) TestBatteryMultiMountSameHost(c *C) {
	outerCount := 5
	count := 15

	for i := 0; i < outerCount; i++ {
		syncChan := make(chan struct{}, count)

		for x := 0; x < count; x++ {
			go func(x int) {
				defer func() { syncChan <- struct{}{} }()
				c.Assert(s.createVolume("mon0", "policy1", fmt.Sprintf("test%02d", x), nil), IsNil)
				dockerCmd := fmt.Sprintf("docker run -d -v policy1/test%02d:/mnt alpine sleep 10m", x)
				out, err := s.vagrant.GetNode("mon0").RunCommandWithOutput(dockerCmd)
				c.Assert(err, IsNil)
				second, err := s.mon0cmd(dockerCmd)
				log.Debug(second, err)
				c.Assert(err, IsNil)

				if cephDriver() {
					_, err = s.mon0cmd(fmt.Sprintf("mount | grep rbd | grep -q policy1.test%02d", x))
					c.Assert(err, IsNil)
					out2, err := s.mon0cmd(fmt.Sprintf("docker exec %s ls /mnt", strings.TrimSpace(out)))
					c.Assert(err, IsNil)
					c.Assert(strings.TrimSpace(out2), Equals, "lost+found")
				}

				out3, err := s.mon0cmd(fmt.Sprintf("docker rm -f %s", strings.TrimSpace(out)))
				if err != nil {
					log.Error(strings.TrimSpace(out3))
				}
				c.Assert(err, IsNil)
			}(x)
		}

		for x := 0; x < count; x++ {
			<-syncChan
		}

		if cephDriver() {
			_, err := s.mon0cmd("mount | grep -q rbd")
			c.Assert(err, IsNil)
		}

		// FIXME netplugin is broken
		c.Assert(s.restartNetplugin(), IsNil)
		c.Assert(s.clearContainers(), IsNil)
		c.Assert(s.restartNetplugin(), IsNil)

		purgeChan := make(chan error, count)
		for x := 0; x < count; x++ {
			go func(x int) { purgeChan <- s.purgeVolume("mon0", "policy1", fmt.Sprintf("test%02d", x), true) }(x)
		}

		var errs int

		for x := 0; x < count; x++ {
			err := <-purgeChan
			if err != nil {
				log.Error(err)
				errs++
			}
		}

		c.Assert(errs, Equals, 0)
	}
}

func (s *systemtestSuite) TestBatteryParallelMount(c *C) {
	// unlocked will be set at the end of the routine. We repeat this test for
	// the NFS driver in unlocked mode to ensure it is not taking locks.
	var unlocked bool

repeat:
	if unlocked {
		log.Info("NFS unlocked test proceeding")
		out, err := s.uploadIntent("policy1", "unlocked")
		c.Assert(err, IsNil, Commentf(out))
	}

	type output struct {
		out    string
		err    error
		volume string
	}

	nodes := s.vagrant.GetNodes()
	outerCount := 5
	count := 15

	for outer := 0; outer < outerCount; outer++ {
		outputChan := make(chan output, len(nodes)*count)

		for x := 0; x < count; x++ {
			go func(nodes []vagrantssh.TestbedNode, x int) {
				for _, node := range nodes {
					c.Assert(s.createVolume(node.GetName(), "policy1", fmt.Sprintf("test%02d", x), nil), IsNil)
				}

				for _, node := range nodes {
					go func(node vagrantssh.TestbedNode, x int) {
						out, err := s.dockerRun(node.GetName(), false, true, fmt.Sprintf("policy1/test%02d", x), "sleep 10m")
						outputChan <- output{out, err, fmt.Sprintf("policy1/test%02d", x)}
					}(node, x)
				}
			}(nodes, x)
		}

		var errs int

		for i := 0; i < len(nodes)*count; i++ {
			output := <-outputChan
			if output.err != nil {
				log.Debug(output.out)
				errs++
			}

			//log.Infof("%q: %s", output.volume, output.out)
		}

		errCount := count * (len(nodes) - 1)
		if unlocked {
			// if we have no locking to stop us, we will have no errors.
			errCount = 0
		}

		c.Assert(errs, Equals, errCount)
		c.Assert(s.clearContainers(), IsNil)

		purgeChan := make(chan error, count)
		for x := 0; x < count; x++ {
			go func(x int) { purgeChan <- s.purgeVolume("mon0", "policy1", fmt.Sprintf("test%02d", x), true) }(x)
		}

		errs = 0

		for x := 0; x < count; x++ {
			err := <-purgeChan
			if err != nil {
				log.Error(err)
				errs++
			}
		}

		c.Assert(errs, Equals, 0)
		c.Assert(s.restartDocker(), IsNil)
	}

	if nfsDriver() && !unlocked {
		unlocked = true
		goto repeat
	}
}

func (s *systemtestSuite) TestBatteryParallelCreate(c *C) {
	nodes := s.vagrant.GetNodes()
	count := 15
	outcount := 5
	outwg := sync.WaitGroup{}

	for outer := 0; outer < outcount; outer++ {
		for x := 0; x < count; x++ {
			outwg.Add(1)
			go func(nodes []vagrantssh.TestbedNode, x int) {
				defer outwg.Done()
				wg := sync.WaitGroup{}
				errChan := make(chan error, len(nodes))

				for i := range rand.Perm(len(nodes)) {
					wg.Add(1)
					go func(i, x int) {
						defer wg.Done()
						node := nodes[i]
						log.Infof("Creating image policy1/test%02d on %q", x, node.GetName())

						var opt string

						if nfsDriver() {
							opt = fmt.Sprintf("--opt mount=%s:policy1/test%02d", s.mon0ip, x)
						}

						_, err := node.RunCommandWithOutput(fmt.Sprintf("volcli volume create policy1/test%02d %s", x, opt))
						errChan <- err
					}(i, x)
				}

				var errs int

				wg.Wait()

				for i := 0; i < len(nodes); i++ {
					err := <-errChan
					if err != nil {
						errs++
					}
				}

				if nfsDriver() {
					c.Assert(errs, Equals, 0)
				} else {
					c.Assert(errs, Equals, 2)
				}
			}(nodes, x)
		}

		outwg.Wait()

		errChan := make(chan error, count)
		for x := 0; x < count; x++ {
			go func(x int) { errChan <- s.purgeVolume("mon0", "policy1", fmt.Sprintf("test%02d", x), true) }(x)
		}

		var realErr error

		for x := 0; x < count; x++ {
			err := <-errChan
			if err != nil {
				realErr = err
			}
		}

		c.Assert(realErr, IsNil)

		if cephDriver() {
			out, err := s.mon0cmd("sudo rbd ls")
			c.Assert(err, IsNil)
			c.Assert(out, Equals, "")
		}
	}
}
