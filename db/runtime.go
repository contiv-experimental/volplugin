package db

import (
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
)

// NewRuntimeOptions creates a new runtime options struct with a path so it can
// be found or written to.
func NewRuntimeOptions(policy, volume string) *RuntimeOptions {
	return &RuntimeOptions{policyName: policy, volumeName: volume}
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
