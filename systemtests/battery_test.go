package systemtests

import (
	"fmt"
	"os"
	"strings"
	"sync"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/vagrantssh"
)

func (s *systemtestSuite) TestBatteryMultiMountSameHost(c *C) {
	c.Skip("Can't be run until docker fixes this bug")
	count := 25
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

	for x := 0; x < count; x++ {
		c.Assert(<-errChan, NotNil)
	}
}
func (s *systemtestSuite) TestBatteryParallelMount(c *C) {
	nodes := s.vagrant.GetNodes()
	count := 10

	if os.Getenv("BIG") != "" {
		count = 50
	}

	outwg := sync.WaitGroup{}
	for x := 0; x < count; x++ {
		outwg.Add(1)
		go func(nodes []vagrantssh.TestbedNode, x int) {
			defer outwg.Done()
			wg := sync.WaitGroup{}
			errChan := make(chan error, len(nodes))

			for _, node := range nodes {
				c.Assert(s.createVolume(node.GetName(), "policy1", fmt.Sprintf("test%d", x), nil), IsNil)
			}

			contID := ""
			var contNode *vagrantssh.TestbedNode

			for _, node := range nodes {
				wg.Add(1)
				go func(node vagrantssh.TestbedNode, x int) {
					log.Infof("Running alpine container for %d on %q", x, node.GetName())

					if out, err := node.RunCommandWithOutput(fmt.Sprintf("docker run -itd -v policy1/test%d:/mnt alpine sleep 10m", x)); err != nil {
						errChan <- err
					} else {
						contID = strings.TrimSpace(out)
						contNode = &node
					}

					wg.Done()
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
			c.Assert(errs, Equals, len(nodes)-1)
			log.Infof("Removing containers for %d: %s", x, contID)
			out, err := (*contNode).RunCommandWithOutput(fmt.Sprintf("docker rm -f %s", contID))
			if err != nil {
				log.Error(out)
			}
			c.Assert(err, IsNil)
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
}

func (s *systemtestSuite) TestBatteryParallelCreate(c *C) {
	nodes := s.vagrant.GetNodes()
	outwg := sync.WaitGroup{}
	count := 10

	if os.Getenv("BIG") != "" {
		count = 50
	}

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

					if out, err := node.RunCommandWithOutput(fmt.Sprintf("volcli volume create policy1/test%d", x)); err != nil {
						log.Error(out)
						log.Error(err)
						errChan <- err
					}
				}(node, x)
			}

			wg.Wait()

			var errs int

			for i := 0; i < len(nodes); i++ {
				select {
				case err := <-errChan:
					log.Errorf("Processing %d: %v", x, err)
					errs++
				default:
				}
			}

			c.Assert(errs, Equals, 0)
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
}
