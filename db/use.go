package db

import (
	"fmt"
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
)

// Use conforms to db.Lock and is used to manage volumes in volplugin.
type Use struct {
	Volume    string `json:"volume"`
	UseOwner  string `json:"owner"`
	UseReason string `json:"reason"`
}

func returnUse(reason, owner string, v *Volume) *Use {
	return &Use{Volume: v.String(), UseOwner: owner, UseReason: reason}
}

// NewUse returns a generic use for a volume, suitable for `Get` calls via the
// Entity interface.
func NewUse(v *Volume) *Use {
	return &Use{Volume: v.String()}
}

// NewCreateOwner returns a lock for a create operation on a volume.
func NewCreateOwner(owner string, v *Volume) *Use {
	return returnUse("Create", owner, v)
}

// NewRemoveOwner returns a lock for a remove operation on a volume.
func NewRemoveOwner(owner string, v *Volume) *Use {
	return returnUse("Remove", owner, v)
}

// NewMountOwner returns a properly formatted *Use. owner is typically a hostname.
func NewMountOwner(owner string, v *Volume) *Use {
	return returnUse("Use", owner, v)
}

// Prefix returns the path under which this data should be stored.
func (m *Use) Prefix() string {
	return "users/volume"
}

// Path returns the volume name.
func (m *Use) Path() (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}

	return strings.Join([]string{m.Prefix(), m.Volume}, "/"), nil
}

// Reason is the reason for taking the lock.
func (m *Use) Reason() string {
	return m.UseReason
}

// Owner is the owner of this lock.
func (m *Use) Owner() string {
	return m.UseOwner
}

// Copy copies a use and returns it.
func (m *Use) Copy() Entity {
	m2 := *m
	return &m2
}

// SetKey sets the volume name from the key, and returns an error if necessary.
func (m *Use) SetKey(key string) error {
	m.Volume = strings.Trim(strings.TrimPrefix(key, m.Prefix()), "/")

	return m.Validate()
}

// Validate does nothing on use locks.
func (m *Use) Validate() error {
	parts := strings.Split(m.Volume, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return errors.InvalidVolume.Combine(errored.New(m.Volume))
	}

	return nil
}

func (m *Use) String() string {
	return fmt.Sprintf("%q: owner: %q; reason %q", m.Volume, m.Owner(), m.Reason())
}

// Hooks returns an empty struct.
func (m *Use) Hooks() *Hooks {
	return &Hooks{}
}
