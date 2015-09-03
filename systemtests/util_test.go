package systemtests

import (
	"fmt"
	"strings"

	utils "github.com/contiv/systemtests-utils"
)

func iterateNodes(fn func(utils.TestbedNode) error) error {
	for _, node := range vagrant.GetNodes() {
		// note that we do all of our work on the monitor instances, so the osd's
		// are left alone
		if strings.HasPrefix(node.GetName(), "mon") {
			if err := fn(node); err != nil {
				return fmt.Errorf(`Error: "%v" on host: %q"`, err, node.GetName())
			}
		}
	}

	return nil
}

func runSSH(cmd string) error {
	return iterateNodes(func(node utils.TestbedNode) error {
		return node.RunCommand(cmd)
	})
}
