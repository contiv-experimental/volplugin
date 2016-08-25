package db

// Node is an abstraction of a KVPair (consul) or *Node (etcd). It is designed
// to be used by drivers, not users.
type Node struct {
	Key   string
	Value []byte
	Dir   bool
	Nodes []*Node
}
