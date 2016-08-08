package entities

import (
	"strings"

	"github.com/docker/go-units"
)

// ActualSize returns the size of the volume as an integer of megabytes.
func (co *CreateOptions) ActualSize() (uint64, error) {
	sizeStr := co.Size

	if strings.TrimSpace(sizeStr) == "" {
		sizeStr = "0"
	}

	size, err := units.FromHumanSize(sizeStr)
	// MB is the base unit for RBD
	return uint64(size) / units.MB, err
}
