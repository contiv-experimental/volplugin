package systemtests

import (
	"fmt"
	"strings"
	"sync"
	"time"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/vagrantssh"
)

func (s *systemtestSuite) TestBatteryMultiMountSameHost(c *C) {
	c.Skip("Can't be run until docker fixes this bug")
	count := 15
	errChan := make(chan error, count)

	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)
	dockerCmd := "docker run -d -v policy1/test:/mnt alpine sleep 10m"
	c.Assert(s.vagrant.GetNode("mon0").RunCommand(dockerCmd), IsNil)

	for x := 0; x < count; x++ {
		go func() {
			dockerCmd := "docker run -d -v policy1/test:/mnt alpine sleep 10m"
			errChan <- s.vagrant.GetNode("mon0").RunCommand(dockerCmd)
		}()
	}

	var realErr error

	for x := 0; x < count; x++ {
		err := <-errChan
		if err != nil {
			realErr = err
		}
	}

	c.Assert(realErr, IsNil)
}
func (s *systemtestSuite) TestBatteryParallelMount(c *C) {
	nodes := s.vagrant.GetNodes()
	outerCount := 5
	count := 15

	for outer := 0; outer < outerCount; outer++ {
		syncChan := make(chan struct{}, len(nodes)*count)
		errChan := make(chan error, len(nodes)*count)  // two potential errors per goroutine
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
		// XXX docker seems to ignore the new create if it happens too quickly. File a
		// bug for this.
		c.Assert(s.restartDocker(), IsNil)
		c.Assert(s.clearContainers(), IsNil)
		time.Sleep(1 * time.Second)
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
