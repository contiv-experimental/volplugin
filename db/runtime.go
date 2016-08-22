package db

import (
	"path"
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
)

// NewRuntimeOptions creates a new runtime options struct with a path so it can
// be found or written to.
func NewRuntimeOptions(policy, volume string) *RuntimeOptions {
	return &RuntimeOptions{policyName: policy, volumeName: volume}
}

// Policy returns the name of the policy associated with these runtime options
func (ro *RuntimeOptions) Policy() string {
	return ro.policyName
}

// Volume returns the name of the volume associated with these runtime options
func (ro *RuntimeOptions) Volume() string {
	return ro.volumeName
}

func (ro *RuntimeOptions) String() string {
	return path.Join(ro.Policy(), ro.Volume())
}

// SetKey sets the key for the runtime options. Needed for retrieval.
func (ro *RuntimeOptions) SetKey(key string) error {
	suffix := strings.Trim(strings.TrimPrefix(key, rootRuntimeOptions), "/")
	parts := strings.Split(suffix, "/")
	if len(parts) != 2 {
		return errors.InvalidDBPath.Combine(errored.Errorf("Args to SetKey for RuntimeOptions were invalid: %v", key))
	}

	if parts[0] == "" || parts[1] == "" {
		return errors.InvalidDBPath.Combine(errored.Errorf("One part of key %v in RuntimeOptions was empty: %v", key, parts))
	}

	ro.policyName = parts[0]
	ro.volumeName = parts[1]

	return nil
}

// Copy returns a copy of this RuntimeOptions.
func (ro *RuntimeOptions) Copy() Entity {
	r := *ro
	return &r
}

// Prefix returns the path of the base directory where these entities are stored.
func (ro *RuntimeOptions) Prefix() string {
	return rootRuntimeOptions
}

// Hooks returns nothing for RuntimeOptions.
func (ro *RuntimeOptions) Hooks() *Hooks {
	return &Hooks{}
}

// Path returns a combined path of the runtime payload.
func (ro *RuntimeOptions) Path() (string, error) {
	if ro.policyName == "" || ro.volumeName == "" {
		return "", errors.InvalidDBPath.Combine(errored.Errorf("empty policy or volume name for runtime options"))
	}

	return strings.Join([]string{ro.Prefix(), ro.policyName, ro.volumeName}, "/"), nil
}

// Validate the Runtime Options.
func (ro *RuntimeOptions) Validate() error {
	if err := validateJSON(RuntimeSchema, ro); err != nil {
		return errors.ErrJSONValidation.Combine(err)
	}

	return nil
}
