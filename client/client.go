package client

import (
	"time"

	"golang.org/x/net/context"
)

// ExistType is used to indicate whether or not a value can be operated on due
// to the state of the existing node.
type ExistType uint

var toInitialize = []EntityCollection{}

const (
	// PrevIgnore ignores any previous content, the default.
	PrevIgnore = 0
	// PrevExist only performs the operation if the node already exists.
	PrevExist = iota
	// PrevNoExist only performs the operation if the node does not exist.
	PrevNoExist = iota
	// PrevValue is used to perform CAS operations. The value must be equivalent if the node exists.
	PrevValue = iota
)

// Client is the basic interface to a key-value store with some common properties:
// * Supports watches
// * Supports CAS operations
type Client interface {
	Get(context.Context, *Pather, *GetOptions) (*Node, error)
	Set(context.Context, *Pather, []byte, *SetOptions) error
	Delete(context.Context, *Pather, *DeleteOptions) error
	Watch(*Pather, chan interface{}, bool, func(*Node, chan interface{}) error) (chan struct{}, chan error)
	WatchStop(*Pather)
	Dump(string) (string, error)
	Path() *Pather
}

// GetOptions are the retrieve options our abstractions support. These
// properties will be translated on a per-driver basis to the appropriate
// idioms.
type GetOptions struct {
	Recursive bool
	Sort      bool // FIXME consul might not have this; is it necessary for anything?
}

// SetOptions are the setting options our abstractions support. These
// properties will be translated on a per-driver basis to the appropriate
// idioms.
type SetOptions struct {
	Exist ExistType
	Value []byte
	Dir   bool
	TTL   time.Duration
}

// DeleteOptions are the removal options our abstractions support. These
// properties will be translated on a per-driver basis to the appropriate
// idioms.
type DeleteOptions struct {
	Dir       bool
	Recursive bool
	Value     []byte
}

// EntityClient is a client that is capable of performing events for a given
// class of Entity.
type EntityClient interface {
	Path() *Pather
	Get(string) (Entity, error)
	Watch(string) (chan interface{}, chan error, error)
	WatchStop(string) error
	Publish(Entity) error
	Delete(string) error
	Collection() EntityCollection
}

// EntityCollection is a sub-hierarchy of entities which live under this one.
type EntityCollection interface {
	WatchAll() (chan interface{}, chan error)
	List() ([]Entity, error)
}

// Entity is an encapsulation of a type in our database. It is used to marshal
// content and provide the path as to which it will be written.
type Entity interface {
	Path(*Pather) string
	Payload() ([]byte, error)
	// FIXME add this
	// Validate() error
}

// Node is the abstract representation of a key/value node in the database.
type Node struct {
	Key   string
	Dir   bool
	Value []byte
	Nodes []*Node
	// XXX there are also other keys that we're not using in etcd such as the index keys and TTL (for read)
}
