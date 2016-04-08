package systemtests

import (
	"fmt"
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
		syncChan := make(chan struct{})

		for x := 0; x < count; x++ {
			go func(x int) {
				defer func() { syncChan <- struct{}{} }()
				c.Assert(s.createVolume("mon0", "policy1", fmt.Sprintf("test%d", x), nil), IsNil)
				dockerCmd := fmt.Sprintf("run -d -v policy1/test%d:/mnt alpine sleep 10m", x)
				out, err := s.docker(dockerCmd)
				c.Assert(err, IsNil)
				_, err = s.docker(dockerCmd)
				c.Assert(err, NotNil)
				_, err = s.mon0cmd(fmt.Sprintf("mount | grep rbd | grep -q policy1.test%d", x))
				c.Assert(err, IsNil)
				out2, err := s.docker(fmt.Sprintf("exec %s ls /mnt", strings.TrimSpace(out)))
				c.Assert(err, IsNil)
				c.Assert(strings.TrimSpace(out2), Equals, "lost+found")
				out3, err := s.docker(fmt.Sprintf("rm -f %s", strings.TrimSpace(out)))
				if err != nil {
					log.Info(strings.TrimSpace(out3))
				}
				c.Assert(err, IsNil)
			}(x)
		}

		for x := 0; x < count; x++ {
			<-syncChan
		}

		// FIXME netplugin is broken
		c.Assert(s.restartNetplugin(), IsNil)
		c.Assert(s.clearContainers(), IsNil)
		c.Assert(s.restartNetplugin(), IsNil)

		purgeChan := make(chan error, count)
		for x := 0; x < count; x++ {
			go func(x int) { purgeChan <- s.purgeVolume("mon0", "policy1", fmt.Sprintf("test%d", x), true) }(x)
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
	nodes := s.vagrant.GetNodes()
	outerCount := 5
	count := 15

	for outer := 0; outer < outerCount; outer++ {
		syncChan := make(chan struct{}, len(nodes)*count)
		errChan := make(chan error, len(nodes)*count)
		outChan := make(chan string, len(nodes)*count) // diagnostics

		for x := 0; x < count; x++ {
			go func(nodes []vagrantssh.TestbedNode, x int) {
				for _, node := range nodes {
					c.Assert(s.createVolume(node.GetName(), "policy1", fmt.Sprintf("test%d", x), nil), IsNil)
				}

				contID := ""
				var contNode *vagrantssh.TestbedNode
				containerSync := make(chan struct{}, len(nodes))

				for _, node := range nodes {
					go func(node vagrantssh.TestbedNode, x int) {
						defer func() { syncChan <- struct{}{} }()
						defer func() { containerSync <- struct{}{} }()
						log.Infof("Running alpine container for %d on %q", x, node.GetName())

						if out, err := node.RunCommandWithOutput(fmt.Sprintf("docker run -itd -v policy1/test%d:/mnt alpine sleep 10m", x)); err != nil {
							outChan <- out
							errChan <- err
						} else {
							contID = strings.TrimSpace(out)
							contNode = &node
							errChan <- nil
						}
					}(node, x)
				}

				for i := 0; i < len(nodes); i++ {
					<-containerSync
				}

				c.Assert(contNode, NotNil)

				log.Infof("Removing containers for %d (host %q): %s", x, (*contNode).GetName(), contID)
				out, err := (*contNode).RunCommandWithOutput(fmt.Sprintf("docker rm -f %s", contID))
				if err != nil {
					log.Error(out)
				}
				c.Assert(err, IsNil)
			}(nodes, x)
		}

		for i := 0; i < len(nodes)*count; i++ {
			<-syncChan
		}

		var errs int

		for i := 0; i < len(nodes)*count; i++ {
			err := <-errChan
			if err != nil {
				errs++
			}
		}

		if errs != count*(len(nodes)-1) {
			for i := 0; i < len(nodes)*count; i++ {
				select {
				case out := <-outChan:
					log.Error(out)
				default:
				}
			}
			c.Fail()
		}

		c.Assert(s.clearContainers(), IsNil)

		purgeChan := make(chan error, count)
		for x := 0; x < count; x++ {
			go func(x int) { purgeChan <- s.purgeVolume("mon0", "policy1", fmt.Sprintf("test%d", x), true) }(x)
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
}

func (s *systemtestSuite) TestBatteryParallelCreate(c *C) {
	nodes := s.vagrant.GetNodes()
	outwg := sync.WaitGroup{}
	count := 15

	for outer := 0; outer < 5; outer++ {
		for x := 0; x < count; x++ {
			outwg.Add(1)
			go func(nodes []vagrantssh.TestbedNode, x int) {
				defer outwg.Done()
				wg := sync.WaitGroup{}
				errChan := make(chan error, len(nodes))

				for _, node := range nodes {
					wg.Add(1)
					go func(node vagrantssh.TestbedNode, x int) {
						defer wg.Done()
						log.Infof("Creating image policy1/test%d on %q", x, node.GetName())

						if _, err := node.RunCommandWithOutput(fmt.Sprintf("volcli volume create policy1/test%d", x)); err != nil {
							errChan <- err
						}
					}(node, x)
				}

				wg.Wait()

				var errs int

				for i := 0; i < len(nodes); i++ {
					select {
					case <-errChan:
						errs++
					default:
					}
				}

				c.Assert(errs, Equals, 2)
			}(nodes, x)
		}

		outwg.Wait()

		errChan := make(chan error, count)
		for x := 0; x < count; x++ {
			go func(x int) { errChan <- s.purgeVolume("mon0", "policy1", fmt.Sprintf("test%d", x), true) }(x)
		}

		var realErr error

		for x := 0; x < count; x++ {
			err := <-errChan
			if err != nil {
				realErr = err
			}
		}

		c.Assert(realErr, IsNil)

		out, err := s.mon0cmd("sudo rbd ls")
		c.Assert(err, IsNil)
		c.Assert(out, Equals, "")
	}
}
