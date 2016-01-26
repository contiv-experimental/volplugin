package systemtests

import (
	"fmt"
	"sync"

	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/vagrantssh"
)

func (s *systemtestSuite) TestBatteryParallelMount(c *C) {
	nodes := s.vagrant.GetNodes()

	outwg := sync.WaitGroup{}
	for x := 0; x < 10; x++ {
		outwg.Add(1)
		go func(nodes []vagrantssh.TestbedNode, x int) {
			defer outwg.Done()
			wg := sync.WaitGroup{}
			errChan := make(chan error, len(nodes))

			for _, node := range nodes {
				c.Assert(s.createVolume(node.GetName(), "tenant1", fmt.Sprintf("test%d", x), nil), IsNil)
			}

			contID := ""
			var contNode *vagrantssh.TestbedNode

			for _, node := range nodes {
				wg.Add(1)
				go func(node vagrantssh.TestbedNode, x int) {
					log.Infof("Running debian container on %q", node.GetName())

					if out, err := node.RunCommandWithOutput(fmt.Sprintf("docker run -itd -v tenant1/test%d:/mnt debian sleep infinity", x)); err != nil {
						errChan <- err
					} else {
						contID = out
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
			c.Assert((*contNode).RunCommand(fmt.Sprintf("docker rm -f %s", contID)), IsNil)
		}(nodes, x)
	}

	outwg.Wait()
}

func (s *systemtestSuite) TestBatteryParallelCreate(c *C) {
	nodes := s.vagrant.GetNodes()
	outwg := sync.WaitGroup{}

	for x := 0; x < 10; x++ {
		outwg.Add(1)
		go func(nodes []vagrantssh.TestbedNode, x int) {
			defer outwg.Done()
			wg := sync.WaitGroup{}
			errChan := make(chan error, len(nodes))

			for _, node := range nodes {
				wg.Add(1)
				go func(node vagrantssh.TestbedNode, x int) {
					defer wg.Done()
					log.Infof("Creating image tenant1/test%d on %q", x, node.GetName())

					if out, err := node.RunCommandWithOutput(fmt.Sprintf("volcli volume create tenant1/test%d", x)); err != nil {
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
				case <-errChan:
					errs++
				default:
				}
			}

			c.Assert(errs, Equals, 0)
		}(nodes, x)
	}

	outwg.Wait()
}
