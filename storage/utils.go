package storage

import (
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
)

// SplitName splits a docker volume name from policy/name safely.
func SplitName(name string) (string, string, error) {
	if strings.Count(name, "/") > 1 {
		return "", "", errors.InvalidVolume.Combine(errored.New(name))
	}

	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.InvalidVolume.Combine(errored.New(name))
	}

	return parts[0], parts[1], nil
}
