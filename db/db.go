package db

import "fmt"

/*

client<->entity relationship:

get/set/delete accept the Path() argument from entity to perform key/value
lookup. Get and set respectively manage Read() and Write() from entity.
*/

// Client provides a method of hooking into our data stores. It processes
// entities (see below) and their pipeline between them and k/v stores such as
// etcd and consul.
type Client interface {
	// Get the entity. Will copy the contents of the JSON into the entity for
	// use.
	Get(Entity) error

	// Set the entity. Will persist the entity to etcd based on the path it
	// provides. (See Path())
	Set(Entity) error

	// Delete the entity from the database. Does nothing to the underlying
	// Object.
	Delete(Entity) error

	// Watch the Entity; just the entity and nothing else. Returns a channel for
	// Entity updates and one for errors.
	Watch(Entity) (chan Entity, chan error) // watch of entity

	// WatchStop stops a watch on a specific Entity.
	WatchStop(Entity) error

	// WatchAll watches the prefix of the entity for changes, watching all of the
	// specific type.
	WatchAll(Entity) (chan Entity, chan error) // watch of subtree

	// WatchAllStop stops a WatchAll.
	WatchAllStop(Entity) error

	// Dump dumps a tarball make with mktemp() to the specified directory.
	Dump(string) (string, error)

	// Prefix returns the base prefix for our keyspace.
	Prefix() string

	// List takes an Entity which it will then populate an []Entity with a list of objects.
	List(Entity) ([]Entity, error)

	// ListPrefix lists all the entities under prefix instead of listing the whole keyspace.
	ListPrefix(string, Entity) ([]Entity, error)
}

// Entity provides an abstraction on our types and how they are persisted to
// k/v stores. They are capable of describing all they need to be written to
// the database, which Client above is trained to do.
type Entity interface {
	// SetKey is used to overwrite or add the key used to store this object. It
	// is expected that the implementor parse the key and add the data it needs
	// to its structs and for generating Path().
	SetKey(string) error

	// Prefix is the base path for all keys; used for watchall and list
	Prefix() string

	// Path to object; used for watch and i/o operations.
	Path() (string, error)

	// Validate the object and ensure it is safe to write, and to use after read.
	Validate() error

	// Copy the object and returns Entity. Typically used to work around
	// interface polymorphism idioms in go.
	Copy() Entity

	// Hooks returns a db.Hooks which contains several functions for the Entity
	// lifecycle, such as pre-set and post-get.
	Hooks() *Hooks

	fmt.Stringer
}

// Hook is the type that represents a client hook in our entities system. Hooks
// are fired at certain client operations (see Hooks below) to cleanup or
// reconcile work.
type Hook func(c Client, obj Entity) error

// Hooks for CRUD operations. See Entity.Hooks()
type Hooks struct {
	PreSet       Hook
	PostSet      Hook
	PreGet       Hook
	PostGet      Hook
	PreDelete    Hook
	PostDelete   Hook
	PreValidate  Hook
	PostValidate Hook
}
