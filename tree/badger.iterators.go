package tree

import (
	"github.com/dgraph-io/badger/v2"
)

type dbIterator struct {
	iter        *badger.Iterator
	tx          *badger.Txn
	rootKeypath Keypath
	scanPrefix  Keypath
	rootItem    *badger.Item
	rootNode    *DBNode
	iterNode    *DBNode
}

// Ensure that dbIterator implements the Iterator interface
var _ Iterator = (*dbIterator)(nil)

func newIteratorFromBadgerIterator(iter *badger.Iterator, keypath Keypath, rootNode *DBNode) *dbIterator {
	rootKeypath := rootNode.addKeyPrefix(rootNode.rootKeypath.Push(keypath))
	scanPrefix := rootKeypath.Copy()
	if len(scanPrefix) != len(rootNode.keyPrefix) {
		scanPrefix = append(scanPrefix, KeypathSeparator[0])
	}

	return &dbIterator{
		iter:        iter,
		tx:          rootNode.tx,
		rootKeypath: rootKeypath,
		scanPrefix:  scanPrefix,
		rootNode:    rootNode,
		iterNode:    &DBNode{tx: rootNode.tx},
	}
}

func (iter *dbIterator) Rewind() {
	rootItem, err := iter.tx.Get(iter.rootKeypath)
	if err == badger.ErrKeyNotFound || err != nil { // Just being explicit here
		// Ignore the root.  Just start iterating from the first actual iterator keypath
		iter.rootItem = nil
		iter.iter.Seek(iter.scanPrefix)
		iter.setNode(iter.iter.Item())
		return
	}
	iter.rootItem = rootItem
	iter.setNode(rootItem)
}

func (iter *dbIterator) SeekTo(keypath Keypath) {
	if keypath.Equals(iter.rootKeypath) {
		iter.Rewind()
		return
	}
	iter.rootItem = nil
	iter.iter.Seek(keypath)
	if iter.Valid() {
		iter.setNode(iter.iter.Item())
	}
}

func (iter *dbIterator) Next() {
	if iter.rootItem != nil {
		iter.rootItem = nil
		if len(iter.rootNode.rmKeyPrefix(iter.rootKeypath)) > 0 {
			iter.iter.Seek(iter.scanPrefix)
		}
	} else {
		iter.iter.Next()
	}
	if !iter.Valid() {
		return
	}
	iter.setNode(iter.iter.Item())
}

func (iter *dbIterator) setNode(item *badger.Item) {
	iter.iterNode.rootKeypath = item.KeyCopy(iter.iterNode.rootKeypath)
	iter.iterNode.rootKeypath = iter.rootNode.rmKeyPrefix(iter.iterNode.rootKeypath)
	if len(iter.iterNode.rootKeypath) == 0 {
		iter.iterNode.rootKeypath = nil
	}
}

func (iter *dbIterator) Node() Node {
	if !iter.Valid() {
		return nil
	}
	return iter.iterNode
}

func (iter *dbIterator) Valid() bool {
	if iter.rootItem != nil {
		return true
	}
	return iter.iter.ValidForPrefix(iter.scanPrefix)
}

func (iter *dbIterator) Close() {
	iter.iter.Close()
}

type dbChildIterator struct {
	*dbIterator
	strippedAbsKeypathParts int
}

func newChildIteratorFromBadgerIterator(iter *badger.Iterator, keypath Keypath, rootNode *DBNode) *dbChildIterator {
	dbIter := newIteratorFromBadgerIterator(iter, keypath, rootNode)
	return &dbChildIterator{
		dbIterator:              dbIter,
		strippedAbsKeypathParts: rootNode.rmKeyPrefix(dbIter.rootKeypath).NumParts(),
	}
}

func (iter *dbChildIterator) Node() Node {
	return iter.dbIterator.Node()
}

func (iter *dbChildIterator) Next() {
	for ; iter.dbIterator.Valid(); iter.dbIterator.Next() {
		node := iter.dbIterator.Node()
		numParts := iter.rootNode.rmKeyPrefix(node.Keypath()).NumParts()

		if numParts == iter.strippedAbsKeypathParts+1 {
			return
		}
	}
}

type dbDepthFirstIterator struct {
	iter        *badger.Iterator
	rootKeypath Keypath
	scanPrefix  Keypath
	tx          *badger.Txn
	rootNode    *DBNode
	iterNode    *DBNode
	done        bool
}

// Ensure that dbDepthFirstIterator implements the Iterator interface
var _ Iterator = (*dbDepthFirstIterator)(nil)

func (iter *dbDepthFirstIterator) Valid() bool {
	return Keypath(iter.iter.Item().Key()).Equals(iter.rootKeypath) || iter.iter.ValidForPrefix(iter.scanPrefix)
}

func (iter *dbDepthFirstIterator) setNode() {
	if !iter.Valid() {
		return
	}
	item := iter.iter.Item()
	iter.iterNode.rootKeypath = iter.rootNode.rmKeyPrefix(item.KeyCopy(iter.iterNode.rootKeypath))
	if len(iter.iterNode.rootKeypath) == 0 {
		iter.iterNode.rootKeypath = nil
	}
}

func (iter *dbDepthFirstIterator) Rewind() {
	iter.done = false
	iter.iter.Rewind()
	iter.setNode()
}

func (iter *dbDepthFirstIterator) SeekTo(keypath Keypath) {
	iter.done = false
	iter.iter.Seek(keypath)
	iter.setNode()
}

func (iter *dbDepthFirstIterator) Node() Node {
	if !iter.Valid() {
		return nil
	}
	return iter.iterNode
}

func (iter *dbDepthFirstIterator) Next() {
	if !iter.Valid() {
		return
	}
	iter.iter.Next()
}

func (iter *dbDepthFirstIterator) Close() {
	iter.iter.Close()
}
