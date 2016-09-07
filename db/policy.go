package db

import (
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
)

// NewPolicy creates a policy struct with the required parameters for using it.
// It will not pass validation.
func NewPolicy(name string) *Policy {
	p := &Policy{Name: name}

	if p.FileSystems == nil {
		p.FileSystems = DefaultFilesystems
	}

	return p
}

// SetKey implements the SetKey entity interface.
func (p *Policy) SetKey(key string) error {
	suffix := strings.Trim(strings.TrimPrefix(key, rootPolicy), "/")
	if strings.Contains(suffix, "/") {
		return errors.InvalidDBPath.Combine(errored.Errorf("Policy name %q contains invalid characters", suffix))
	}

	if suffix == "" {
		return errors.InvalidDBPath.Combine(errored.New("Policy name is empty"))
	}

	p.Name = suffix
	return nil
}

// Prefix returns the path of the base directory where these entities are stored.
func (p *Policy) Prefix() string {
	return rootPolicy
}

// Path returns the path to the policy in the DB.
func (p *Policy) Path() (string, error) {
	if p.Name == "" {
		return "", errored.Errorf("Name is blank for this policy").Combine(errors.InvalidDBPath)
	}

	return strings.Join([]string{p.Prefix(), p.Name}, "/"), nil
}

// Validate validates the policy. Returns error on failure.
func (p *Policy) Validate() error {
	if err := validateJSON(RuntimeSchema, p.RuntimeOptions); err != nil {
		return errors.ErrJSONValidation.Combine(err)
	}

	if err := validateJSON(PolicySchema, p); err != nil {
		return errors.ErrJSONValidation.Combine(err)
	}

	if p.Backends == nil { // backend should be defined and its validated
		backends, ok := DefaultDrivers[p.Backend]

		if !ok {
			return errored.Errorf("Invalid backend: %v", p.Backend)
		}
		p.Backends = backends
	}

	size, err := p.CreateOptions.ActualSize()
	if p.Backends.CRUD != "" && (size == 0 || err != nil) {
		return errored.Errorf("Size set to zero for non-empty CRUD backend %v", p.Backends.CRUD).Combine(err)
	}

	return nil
}

func (p *Policy) String() string {
	return p.Name
}

// Copy returns a deep copy of the policy
func (p *Policy) Copy() Entity {
	p2 := *p

	// XXX backends are special. They are optional and making them empty results in
	// a nil pointer. However, in this copy we don't want to copy a pointer, just
	// the data if it exists.
	if p.Backends != nil {
		b2 := *p.Backends
		p2.Backends = &b2
	}

	if p.RuntimeOptions != nil {
		ro2 := *p.RuntimeOptions
		p2.RuntimeOptions = &ro2
	}

	return &p2
}

// Hooks returns the public hooks this type registers with the client.
func (p *Policy) Hooks() *Hooks {
	return &Hooks{}
}
