package client

import (
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/errors"
)

// Pather knows how to construct k/v store paths.
type Pather struct {
	content   []string
	separator string
	leadSep   bool
}

func newPather(content, separator string, leadSep bool) *Pather {
	return &Pather{
		content:   []string{content},
		separator: separator,
		leadSep:   leadSep,
	}
}

// NewEtcdPather constructs a pather suitable for use with etcd.
func NewEtcdPather(prefix string) *Pather {
	return newPather(prefix, "/", true)
}

// NewConsulPather constructs a pather suitable for use with consul.
func NewConsulPather(prefix string) *Pather {
	return newPather(prefix, "/", false)
}

// String returns the pather's string representation.
func (p *Pather) String() string {
	var sep string
	if p.leadSep {
		sep = p.separator
	}
	return sep + strings.Join(p.content, p.separator)
}

func (p *Pather) validateNames(name []string) error {
	for _, val := range name {
		if strings.Contains(val, "/") {
			return errors.InvalidPath.Combine(errored.New(val))
		}
	}

	return nil
}

// Combine is a path construction utility to prepend the prefix and join all the
// specified items by the separator. Returns a *Pather.
func (p *Pather) Combine(pather *Pather) (*Pather, error) {
	if err := p.validateNames(pather.content); err != nil {
		return nil, err
	}

	return p.Append(pather.content...)
}

// Append appends a set of strings to the end of a copy of this pather, and
// returns the new value.
func (p *Pather) Append(name ...string) (*Pather, error) {
	if err := p.validateNames(name); err != nil {
		return nil, err
	}

	p2 := *p
	p2.content = append(p2.content, name...)
	return &p2, nil
}

// Replace replaces the contents of a pather with new contents provided. This
// can be useful when you don't know the underlying database implementation but
// need to navigate to a new namespace.
func (p *Pather) Replace(name ...string) (*Pather, error) {
	if err := p.validateNames(name); err != nil {
		return nil, err
	}

	p2 := *p
	p2.content = name
	return &p2, nil
}
