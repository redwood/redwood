package state

import (
	"github.com/dgraph-io/badger/v3"
)

type ReusableIterator = reusableIterator
type DBIterator = dbIterator

func (iter *dbIterator) BadgerIter() *badger.Iterator {
	return iter.iter
}

func (t *VersionedDBTree) BadgerDB() *badger.DB {
	return t.db
}

func (n *MemoryNode) Keypaths() []Keypath {
	return n.keypaths
}

func (n *MemoryNode) NodeTypes() map[string]NodeType {
	return n.nodeTypes
}

func (n *MemoryNode) Values() map[string]interface{} {
	return n.values
}

func (n *MemoryNode) ContentLengths() map[string]uint64 {
	return n.contentLengths
}
