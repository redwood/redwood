package tree

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/dgraph-io/badger/v2"
	badgerpb "github.com/dgraph-io/badger/v2/pb"
	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type DBTree struct {
	db       *badger.DB
	filename string
	ctx.Logger
}

func NewDBTree(dbFilename string) (*DBTree, error) {
	opts := badger.DefaultOptions(dbFilename)
	opts.Logger = nil

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &DBTree{db, dbFilename, ctx.NewLogger("db tree")}, nil
}

func (t *DBTree) Close() error {
	return t.db.Close()
}

func (t *DBTree) DeleteDB() error {
	err := t.Close()
	if err != nil {
		return err
	}
	return os.RemoveAll(t.filename)
}

var stateKeyPrefixLen = len(types.ID{}) + 1

func (tx *DBNode) addKeyPrefix(keypath Keypath) Keypath {
	return append(tx.keyPrefix, keypath...)
}

func (tx *DBNode) rmKeyPrefix(keypath Keypath) Keypath {
	return keypath[len(tx.keyPrefix):]
}

func (t *DBTree) makeStateKeyPrefix(version types.ID) []byte {
	// <version>:
	keyPrefix := make([]byte, stateKeyPrefixLen)
	copy(keyPrefix, version.Bytes())
	keyPrefix[len(keyPrefix)-1] = ':'
	return keyPrefix
}

func (t *DBTree) makeIndexKeyPrefix(version types.ID, keypath Keypath, indexName Keypath) []byte {
	// i:<version>:<keypath>:<indexName>:
	// i:deadbeef19482:foo/messages:author:
	return bytes.Join([][]byte{[]byte("i"), version[:], keypath, indexName, []byte{}}, []byte(":"))
}

func (t *DBTree) StateAtVersion(version *types.ID, mutable bool) *DBNode {
	if version == nil {
		version = &CurrentVersion
	}
	var diff *Diff
	if mutable {
		diff = NewDiff()
	}
	return &DBNode{
		tx:        t.db.NewTransaction(mutable),
		keyPrefix: t.makeStateKeyPrefix(*version),
		diff:      diff,
	}
}

func (t *DBTree) IndexAtVersion(version *types.ID, keypath Keypath, indexName Keypath, mutable bool) *DBNode {
	if version == nil {
		version = &CurrentVersion
	}
	return &DBNode{
		tx:        t.db.NewTransaction(mutable),
		keyPrefix: t.makeIndexKeyPrefix(*version, keypath, indexName),
	}
}

func (t *DBTree) Value(version *types.ID, keypathPrefix Keypath, rng *Range) (interface{}, bool, error) {
	node := t.StateAtVersion(version, false)
	defer node.Close()
	return node.Value(keypathPrefix, rng)
}

type DBNode struct {
	tx          *badger.Txn
	diff        *Diff
	keyPrefix   []byte
	rootKeypath Keypath
	rng         *Range
}

// Ensure DBNode implements the Node interface
var _ Node = (*DBNode)(nil)

func (tx *DBNode) Close() {
	tx.tx.Discard()
}

func (tx *DBNode) Save() error {
	var err error
	var wg sync.WaitGroup
	wg.Add(1)
	tx.tx.CommitWith(func(innerErr error) {
		err = innerErr
		wg.Done()
	})
	return err
}

func (tx *DBNode) Keypath() Keypath {
	return tx.rootKeypath
}

func (tx *DBNode) NodeAt(keypath Keypath, rng *Range) Node {
	return &DBNode{tx: tx.tx, rootKeypath: tx.rootKeypath.Push(keypath), rng: tx.rng, keyPrefix: tx.keyPrefix, diff: tx.diff}
}

func (n *DBNode) ParentNodeFor(keypath Keypath) (Node, Keypath) {
	return n, keypath
}

func (tx *DBNode) Subkeys() []Keypath {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchSize = 10
	iter := tx.tx.NewIterator(opts)
	defer iter.Close()

	startKeypath := append(tx.addKeyPrefix(tx.rootKeypath), KeypathSeparator[0])

	var keypaths []Keypath
	keypathsMap := make(map[string]struct{})
	for iter.Seek(startKeypath); iter.ValidForPrefix(startKeypath); iter.Next() {
		item := iter.Item()
		absKeypath := Keypath(item.Key())
		subkey := tx.rmKeyPrefix(absKeypath).RelativeTo(tx.rootKeypath).Part(0)
		_, exists := keypathsMap[string(subkey)]
		if !exists && len(subkey) > 0 {
			keypaths = append(keypaths, subkey)
			keypathsMap[string(subkey)] = struct{}{}
		}
	}
	return keypaths
}

func (tx *DBNode) NodeInfo(keypath Keypath) (NodeType, ValueType, uint64, error) {
	item, err := tx.tx.Get(tx.addKeyPrefix(tx.rootKeypath.Push(keypath)))
	if err == badger.ErrKeyNotFound {
		return 0, 0, 0, errors.Wrap(types.Err404, tx.addKeyPrefix(tx.rootKeypath.Push(keypath)).String())
	} else if err != nil {
		return 0, 0, 0, errors.WithStack(err)
	}

	var nodeType NodeType
	var valueType ValueType
	var length uint64
	err = item.Value(func(bs []byte) error {
		nodeType, valueType, length, _, err = decodeNode(bs)
		return err
	})
	if err != nil {
		return 0, 0, 0, errors.WithStack(err)
	}
	return nodeType, valueType, length, nil
}

func (tx *DBNode) Exists(keypath Keypath) (bool, error) {
	_, err := tx.tx.Get(tx.addKeyPrefix(tx.rootKeypath.Push(keypath)))
	if err == badger.ErrKeyNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (tx *DBNode) Value(keypathPrefix Keypath, rng *Range) (interface{}, bool, error) {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchSize = 10
	iter := tx.tx.NewIterator(opts)
	defer iter.Close()

	return tx.value(keypathPrefix, rng, iter)
}

func (tx *DBNode) value(keypathPrefix Keypath, rng *Range, iter *badger.Iterator) (_ interface{}, _ bool, err error) {
	defer annotate(&err, "Value (keypath: %v, range: %v)", keypathPrefix, rng)

	if rng == nil {
		rng = tx.rng
	} else if rng != nil && tx.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return nil, false, ErrInvalidRange
	}

	keypathPrefix = tx.addKeyPrefix(tx.rootKeypath.Push(keypathPrefix))

	item, err := tx.tx.Get(keypathPrefix)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			err = errors.Wrapf(types.Err404, "(keypath: %v)", keypathPrefix)
		}
		return nil, false, err
	}

	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, false, err
	}

	rootNodeType, valueType, length, data, err := decodeNode(val)
	if err != nil {
		return nil, false, err
	}
	defer annotate(&err, "(root nodeType: %v, root valueType: %v, root length: %v, root data: %v)", rootNodeType, valueType, length, string(data))

	if rootNodeType == NodeTypeValue {
		v, err := decodeGoValue(rootNodeType, valueType, length, rng, data)
		if err != nil {
			return nil, false, err
		}
		return v, true, nil
	}

	var startKeypath Keypath
	var endKeypath Keypath
	var subkeyIdx uint64
	var endIdx uint64

	goValues := make(map[string]interface{})

	if rootNodeType == NodeTypeMap {
		goValues[""] = make(map[string]interface{})

		if rng != nil {
			if !rng.ValidForLength(length) {
				return nil, false, ErrInvalidRange
			} else if rng.Size() == 0 {
				return map[string]interface{}{}, true, nil
			}
			var startIdx uint64
			startIdx, endIdx = rng.IndicesForLength(length)
			startKeypath = tx.keypathAtMapIndex(keypathPrefix, startIdx, iter)
			subkeyIdx = startIdx

		} else {
			startKeypath = append(keypathPrefix, KeypathSeparator[0])
		}

	} else if rootNodeType == NodeTypeSlice {
		goValues[""] = make([]interface{}, length)

		if rng != nil {
			if !rng.ValidForLength(length) {
				return nil, false, ErrInvalidRange
			} else if rng.Size() == 0 {
				return []interface{}{}, true, nil
			}
			startIdx, endIdx := rng.IndicesForLength(length)
			startKeypath = keypathPrefix.PushIndex(startIdx)
			endKeypath = keypathPrefix.PushIndex(endIdx)

		} else {
			startKeypath = append(keypathPrefix, KeypathSeparator[0])
		}
	}

	var validPrefix Keypath
	if len(tx.rmKeyPrefix(keypathPrefix)) > 0 {
		validPrefix = append(keypathPrefix, KeypathSeparator[0])
	} else {
		validPrefix = keypathPrefix
	}

	var valueBuf []byte
	var prevKeypath Keypath
	var shouldStop bool
	for iter.Seek(startKeypath); iter.ValidForPrefix(validPrefix) && !shouldStop; iter.Next() {
		item := iter.Item()
		absKeypath := Keypath(item.Key())

		// If we have a range, we have to figure out when to stop iterating
		if rng != nil {
			if rootNodeType == NodeTypeSlice && absKeypath.Equals(endKeypath) {
				shouldStop = true

			} else if rootNodeType == NodeTypeMap {
				// If we're ranging over a map, we check to see if we've reached the end of the range.
				// Then, we check the first part of the relative keypath to see if it's different from
				// the previous keypath.  If so, increment the subkey counter.
				if prevKeypath != nil {
					if !absKeypath.Part(0).Equals(prevKeypath.Part(0)) {
						subkeyIdx++
					}
					if subkeyIdx == endIdx {
						break
					}
				}
				prevKeypath = absKeypath.Copy()
			}
		}

		relKeypath := absKeypath.RelativeTo(keypathPrefix)

		// If we're ranging over a slice, transpose its indices to start from 0.
		if len(relKeypath) > 0 && rootNodeType == NodeTypeSlice && rng != nil {
			relKeypath = relKeypath.Copy()
			oldIdx := DecodeSliceIndex(relKeypath[:8])
			newIdx := uint64(int64(oldIdx) - rng[0])
			copy(relKeypath[:8], EncodeSliceIndex(newIdx))
		}

		// Decode the value from the DB into a Go value
		var val interface{}
		{
			valueBuf, err = item.ValueCopy(valueBuf)
			if err != nil {
				return nil, false, err
			}

			nodeType, valueType, length, data, err := decodeNode(valueBuf)
			if err != nil {
				return nil, false, err
			}

			// Only apply the passed range to the root value
			var thisRng *Range
			if len(relKeypath) == 0 {
				thisRng = rng
			}

			val, err = decodeGoValue(nodeType, valueType, length, thisRng, data)
			if err != nil {
				return nil, false, err
			}

			switch val.(type) {
			case map[string]interface{}, []interface{}:
				goValues[string(relKeypath)] = val
			default:
				if len(relKeypath) == 0 {
					return val, true, nil
				}
			}
		}

		// Insert the decoded value into its parent
		{
			parentKeypath, key := relKeypath.Pop()
			if len(key) == 0 {
				// This keypath is the root keypath (nil), so there's no parent to insert it into
				continue
			}

			parent := goValues[string(parentKeypath)]
			switch p := parent.(type) {
			case map[string]interface{}:
				p[string(key)] = val
			case []interface{}:
				p[DecodeSliceIndex(key)] = val
			default:
				fmt.Println(prettyJSON(goValues))
				panic(fmt.Sprintf("bad parent type: setting [%v].  %v is (%T) == %v", relKeypath, parentKeypath, parent, parent))
			}
		}
	}

	root, exists := goValues[""]
	return root, exists, nil
}

func (n *DBNode) keypathAtMapIndex(keypathPrefix Keypath, desiredIdx uint64, bIter *badger.Iterator) Keypath {
	iter := newChildIteratorFromBadgerIterator(bIter, n.rmKeyPrefix(keypathPrefix), n)
	defer bIter.Seek(keypathPrefix)

	var idx uint64
	for {
		node := iter.Node()
		if node == nil {
			return nil
		}

		if idx == desiredIdx {
			return node.Keypath()
		}
		iter.Next()
		idx++
	}
	return nil
}

func (tx *DBNode) UintValue(keypath Keypath) (uint64, bool, error) {
	// @@TODO: maybe optimize the case where the keypath holds a complex object
	// that .Value takes a while to decode by first checking the value's type?
	val, exists, err := tx.Value(keypath, nil)
	if err != nil {
		return 0, false, err
	} else if !exists {
		return 0, false, nil
	}
	u, isFloat := val.(uint64)
	if isFloat {
		return u, true, nil
	}
	return 0, false, nil
}

func (tx *DBNode) IntValue(keypath Keypath) (int64, bool, error) {
	// @@TODO: maybe optimize the case where the keypath holds a complex object
	// that .Value takes a while to decode by first checking the value's type?
	val, exists, err := tx.Value(keypath, nil)
	if err != nil {
		return 0, false, err
	} else if !exists {
		return 0, false, nil
	}
	i, isFloat := val.(int64)
	if isFloat {
		return i, true, nil
	}
	return 0, false, nil
}

func (tx *DBNode) FloatValue(keypath Keypath) (float64, bool, error) {
	// @@TODO: maybe optimize the case where the keypath holds a complex object
	// that .Value takes a while to decode by first checking the value's type?
	val, exists, err := tx.Value(keypath, nil)
	if err != nil {
		return 0, false, err
	} else if !exists {
		return 0, false, nil
	}
	f, isFloat := val.(float64)
	if isFloat {
		return f, true, nil
	}
	return 0, false, nil
}

func (tx *DBNode) StringValue(keypath Keypath) (string, bool, error) {
	// @@TODO: maybe optimize the case where the keypath holds a complex object
	// that .Value takes a while to decode by first checking the value's type?
	val, exists, err := tx.Value(keypath, nil)
	if err != nil {
		return "", false, err
	} else if !exists {
		return "", false, nil
	}
	s, isString := val.(string)
	if !isString {
		return "", false, nil
	}
	return s, true, nil
}

func (tx *DBNode) ContentLength() (int64, error) {
	item, err := tx.tx.Get(tx.addKeyPrefix(tx.rootKeypath))
	if err != nil {
		return 0, err
	}

	var val []byte
	item.Value(func(bs []byte) error {
		val = bs
		return nil
	})

	nodeType, valueType, length, _, err := decodeNode(val)
	if err != nil {
		return 0, err
	}

	switch nodeType {
	case NodeTypeMap:
		return 0, nil
	case NodeTypeSlice:
		return int64(length), nil
	case NodeTypeValue:
		switch valueType {
		case ValueTypeString:
			return int64(length), nil
		default:
			return 0, nil
		}
	default:
		return 0, nil
	}
}

func (tx *DBNode) Set(relKeypath Keypath, rng *Range, val interface{}) error {
	if rng == nil {
		rng = tx.rng
	} else if rng != nil && tx.rng != nil {
		panic("unsupported")
	}

	absKeypath := tx.rootKeypath.Push(relKeypath)

	if rng != nil {
		if !rng.Valid() {
			return errors.WithStack(ErrInvalidRange)
		}

		item, err := tx.tx.Get(tx.addKeyPrefix(absKeypath))
		if err == badger.ErrKeyNotFound {
			// @@TODO: ??
			//return errors.WithStack(ErrRangeOverNonSlice)
		} else if err != nil {
			return errors.Errorf("error fetching keypath %v while setting range", absKeypath)
		}

		var encodedVal []byte
		if item != nil {
			encodedVal, err = item.ValueCopy(nil)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		switch spliceVal := val.(type) {
		case string:
			return tx.setRangeString(absKeypath, rng, encodedVal, spliceVal)
		case []interface{}:
			return tx.setRangeSlice(absKeypath, rng, encodedVal, spliceVal)
		default:
			return errors.New("wrong type for splice")
		}
	} else {
		return tx.setNoRange(absKeypath, val)
	}
}

func (tx *DBNode) setRangeString(absKeypath Keypath, rng *Range, encodedVal []byte, spliceVal string) error {
	if len(encodedVal) == 0 {
		encodedVal = []byte("vs")
	}
	_, valueType, length, data, err := decodeNode(encodedVal)
	if err != nil {
		return err
	} else if valueType != ValueTypeString {
		return errors.WithStack(ErrRangeOverNonSlice)
	} else if !rng.ValidForLength(length) {
		return errors.WithStack(ErrInvalidRange)
	}

	startIdx, endIdx := rng.IndicesForLength(length)
	oldVal := data
	newLen := 2 + length - rng.Size() + uint64(len(spliceVal))
	newVal := make([]byte, newLen)
	newVal[0] = 'v'
	newVal[1] = 's'
	copy(newVal[2:], oldVal[:startIdx])
	copy(newVal[2+startIdx:], []byte(spliceVal))
	copy(newVal[2+startIdx+uint64(len(spliceVal)):], oldVal[endIdx:])
	return tx.tx.Set(tx.addKeyPrefix(absKeypath), newVal)
}

func (tx *DBNode) setRangeSlice(absKeypath Keypath, rng *Range, encodedVal []byte, spliceVal []interface{}) (err error) {
	defer annotate(&err, "setRangeSlice")

	nodeType, _, oldLen, _, err := decodeNode(encodedVal)
	if err != nil {
		return err
	} else if nodeType != NodeTypeSlice {
		return ErrRangeOverNonSlice
	} else if !rng.ValidForLength(oldLen) {
		return ErrInvalidRange
	}

	absKeypath = tx.addKeyPrefix(absKeypath)

	newLen := oldLen - rng.Size() + uint64(len(spliceVal))
	shrink := newLen < oldLen
	startIdx, endIdx := rng.IndicesForLength(oldLen)

	// Delete deleted items
	{
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Reverse = false
		opts.Prefix = append(absKeypath, KeypathSeparator[0])

		iter := tx.tx.NewIterator(opts)
		defer iter.Close()

		startKeypath := absKeypath.PushIndex(startIdx)
		endKeypath := absKeypath.PushIndex(endIdx)

		for iter.Seek(startKeypath); iter.ValidForPrefix(opts.Prefix); iter.Next() {
			item := iter.Item()
			keypath := Keypath(item.KeyCopy(nil))
			if keypath.Equals(endKeypath) {
				break
			}

			tx.diff.Remove(tx.rmKeyPrefix(keypath))
			err := tx.tx.Delete(keypath)
			if err != nil {
				return errors.Wrapf(err, "can't delete keypath %v", keypath)
			}
		}
		iter.Close()
	}

	// Shift indices of trailing items
	if newLen != oldLen && oldLen != 0 {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Reverse = !shrink

		scanPrefix := append(absKeypath, KeypathSeparator[0])

		iter := tx.tx.NewIterator(opts)
		defer iter.Close()

		var startKeypath Keypath
		var endKeypath Keypath
		if shrink {
			startKeypath = absKeypath.PushIndex(endIdx)
			endKeypath = absKeypath.PushIndex(oldLen)
		} else {
			startKeypath = absKeypath.PushIndex(oldLen - 1).Push([]byte{0xff})
			endKeypath = absKeypath.PushIndex(startIdx)
		}
		prefixLen := len(scanPrefix)

		var keypathBuf []byte
		for iter.Seek(startKeypath); iter.ValidForPrefix(scanPrefix); iter.Next() {
			item := iter.Item()
			oldKeypath := Keypath(item.KeyCopy(keypathBuf))
			if oldKeypath.Equals(endKeypath) {
				break
			}

			oldIdx := DecodeSliceIndex(oldKeypath[prefixLen : prefixLen+8])
			var newIdx uint64
			if shrink {
				newIdx = oldIdx - (oldLen - newLen)
			} else {
				if oldIdx < startIdx {
					break
				}
				newIdx = oldIdx + (newLen - oldLen)
			}

			valueBuf, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			newKeypath := oldKeypath.Copy()
			copy(newKeypath[prefixLen:prefixLen+8], EncodeSliceIndex(newIdx))

			err = tx.tx.Set(newKeypath, valueBuf)
			if err != nil {
				return errors.Wrapf(err, "can't set keypath %v", newKeypath)
			}
			err = tx.tx.Delete(oldKeypath)
			if err != nil {
				return errors.Wrapf(err, "can't delete keypath %v", oldKeypath)
			}
		}
		iter.Close()
	}

	// Finally, splice in the new values
	err = walkGoValue(spliceVal, func(nodeKeypath Keypath, val interface{}) error {
		nodeKeypath = nodeKeypath.Copy()
		if len(nodeKeypath) == 0 {
			encoded, err := encodeNode(NodeTypeSlice, ValueTypeInvalid, newLen, nil)
			if err != nil {
				return err
			}

			err = tx.tx.Set(absKeypath, encoded)
			if err != nil {
				return err
			}
			return nil
		}

		absNodeKeypath := absKeypath
		oldIdx := DecodeSliceIndex(nodeKeypath[:8])
		newIdx := oldIdx + startIdx
		copy(nodeKeypath[:8], EncodeSliceIndex(newIdx))
		absNodeKeypath = absKeypath.Push(nodeKeypath)

		encoded, err := encodeGoValue(val)
		if err != nil {
			return err
		}

		tx.diff.Add(tx.rmKeyPrefix(absNodeKeypath))

		err = tx.tx.Set(absNodeKeypath, encoded)
		if err != nil {
			return errors.Wrapf(err, "can't set keypath %v", absNodeKeypath)
		}
		return nil
	})
	return err
}

func (tx *DBNode) setNoRange(absKeypath Keypath, value interface{}) error {
	err := tx.Delete(absKeypath.RelativeTo(tx.rootKeypath), nil)
	if err != nil {
		return err
	}

	// Set value types for intermediate keypaths in case they don't exist
	partialKeypath, _ := absKeypath.Pop()
	partialKeypath = tx.addKeyPrefix(partialKeypath)
	for {
		item, err := tx.tx.Get(partialKeypath)
		if err == badger.ErrKeyNotFound || item.IsDeletedOrExpired() {
			encoded, err := encodeNode(NodeTypeMap, ValueTypeInvalid, 1, nil)
			if err != nil {
				return err
			}
			err = tx.tx.Set(partialKeypath, encoded)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			if val[0] == 'v' {
				encoded, err := encodeNode(NodeTypeMap, ValueTypeInvalid, 1, nil)
				if err != nil {
					return err
				}
				err = tx.tx.Set(partialKeypath, encoded)
				if err != nil {
					return err
				}
			}
		}
		if len(partialKeypath) == len(tx.keyPrefix)+len(tx.rootKeypath) {
			break
		}
		partialKeypath, _ = partialKeypath.Pop()
		if partialKeypath == nil {
			partialKeypath = tx.addKeyPrefix(partialKeypath)
		}
	}

	err = walkGoValue(value, func(nodeKeypath Keypath, nodeValue interface{}) error {
		absNodeKeypath := absKeypath
		if len(nodeKeypath) != 0 {
			absNodeKeypath = absKeypath.Push(nodeKeypath)
		}

		if asNode, isNode := nodeValue.(Node); isNode {
			return tx.setNode(absNodeKeypath, asNode)
		}

		// @@TODO: diff logic sometimes overstates the change or duplicates keypaths
		tx.diff.Add(absNodeKeypath)

		encoded, err := encodeGoValue(nodeValue)
		if err != nil {
			return err
		}
		return tx.tx.Set(tx.addKeyPrefix(absNodeKeypath), encoded)
	})
	return err
}

func (tx *DBNode) encodedBytes(keypath Keypath) ([]byte, error) {
	item, err := tx.tx.Get(tx.addKeyPrefix(keypath))
	if err != nil {
		return nil, err
	}
	return item.ValueCopy(nil)
}

func (tx *DBNode) setNode(keypath Keypath, node Node) error {
	iter := node.Iterator(nil, true, 10)
	defer iter.Close()

	prefixedKeypath := tx.addKeyPrefix(tx.rootKeypath.Push(keypath))
	for ; iter.Node() != nil; iter.Next() {
		child := iter.Node()

		var encoded []byte
		var err error
		if asDBNode, isDBNode := node.(interface {
			encodedBytes(keypath Keypath) ([]byte, error)
		}); isDBNode {
			encoded, err = asDBNode.encodedBytes(keypath)

		} else {
			nodeType, valueType, length, err := child.NodeInfo(nil)
			if err != nil {
				return err
			}
			var val interface{}
			if nodeType == NodeTypeValue {
				var exists bool
				val, exists, err = child.Value(nil, nil)
				if err != nil {
					return err
				} else if !exists {
					// @@TODO: this shouldn't happen
					continue
				}
			}
			encoded, err = encodeNode(nodeType, valueType, length, val)
		}

		relKeypath := child.Keypath().RelativeTo(node.Keypath())
		err = tx.tx.Set(prefixedKeypath.Push(relKeypath), encoded)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tx *DBNode) Delete(relKeypath Keypath, rng *Range) error {
	if rng == nil {
		rng = tx.rng
	} else if rng != nil && tx.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return errors.WithStack(ErrInvalidRange)
	}

	relKeypath = tx.addKeyPrefix(tx.rootKeypath.Push(relKeypath))

	item, err := tx.tx.Get(relKeypath)
	if err != nil && err == badger.ErrKeyNotFound {
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	val, err := item.ValueCopy(nil)
	if err != nil {
		return errors.WithStack(err)
	}

	rootNodeType, valueType, length, data, err := decodeNode(val)
	if err != nil {
		return err
	}

	if rootNodeType == NodeTypeValue {
		if rng != nil {
			if valueType != ValueTypeString {
				return errors.WithStack(ErrRangeOverNonSlice)
			} else if !rng.ValidForLength(length) {
				return errors.WithStack(ErrInvalidRange)
			}

			v, err := decodeGoValue(rootNodeType, valueType, length, nil, data)
			if err != nil {
				return err
			}

			startIdx, endIdx := rng.IndicesForLength(length)
			s := v.(string)
			s = s[:startIdx] + s[endIdx:]

			tx.diff.Remove(tx.rmKeyPrefix(relKeypath))
			return tx.Set(relKeypath, nil, s)
		}

		tx.diff.Remove(tx.rmKeyPrefix(relKeypath))
		return tx.tx.Delete(relKeypath)
	}

	// Remove the root element if there's no range
	if rng == nil {
		err := tx.tx.Delete(relKeypath)
		if err != nil {
			return err
		}
		tx.diff.Remove(tx.rmKeyPrefix(relKeypath))
	}

	var startIdx, endIdx uint64
	var startKeypath Keypath
	var endKeypath Keypath
	var validPrefix Keypath

	if rootNodeType == NodeTypeMap {
		if rng != nil {
			return errors.WithStack(ErrRangeOverNonSlice)
		}
		validPrefix = append(relKeypath, KeypathSeparator[0])
		startKeypath = validPrefix

	} else if rootNodeType == NodeTypeSlice {
		if rng != nil {
			if !rng.ValidForLength(length) {
				return errors.WithStack(ErrInvalidRange)
			}
			startIdx, endIdx = rng.IndicesForLength(length)
			startKeypath = relKeypath.PushIndex(startIdx)
			endKeypath = relKeypath.PushIndex(endIdx)

		} else {
			validPrefix = append(relKeypath, KeypathSeparator[0])
			startKeypath = validPrefix
		}
	}

	// Delete child nodes
	{
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false

		iter := tx.tx.NewIterator(opts)
		defer iter.Close()

		var shouldStop bool
		for iter.Seek(startKeypath); iter.ValidForPrefix(validPrefix) && !shouldStop; iter.Next() {
			absKeypath := Keypath(iter.Item().Key())

			if rng != nil && absKeypath.Equals(endKeypath) {
				shouldStop = true
			}

			// This .Copy() is necessary.  See https://github.com/dgraph-io/badger/issues/494
			err := tx.tx.Delete(absKeypath.Copy())
			if err != nil {
				return errors.Wrapf(err, "can't delete keypath %v", absKeypath)
			}

			tx.diff.Remove(tx.rmKeyPrefix(absKeypath))
		}
		iter.Close()
	}

	// Re-number the trailing entries
	if rng != nil && endIdx < length {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true

		iter := tx.tx.NewIterator(opts)
		defer iter.Close()

		startKeypath := relKeypath.PushIndex(endIdx)
		endKeypathPrefix := relKeypath.PushIndex(length)
		validPrefix := relKeypath
		delta := -int64(rng.Size())
		keypathLengthAsParent := len(tx.keyPrefix) + tx.rmKeyPrefix(relKeypath).LengthAsParent()

		var keypathBuf []byte
		for iter.Seek(startKeypath); iter.ValidForPrefix(validPrefix); iter.Next() {
			item := iter.Item()
			oldKeypath := Keypath(item.KeyCopy(keypathBuf))
			if oldKeypath.Equals(endKeypathPrefix) {
				validPrefix = append(endKeypathPrefix, KeypathSeparator[0])
			}

			oldIdx := DecodeSliceIndex(oldKeypath[keypathLengthAsParent : keypathLengthAsParent+8])
			newIdx := uint64(int64(oldIdx) + delta)

			valueBuf, err := item.ValueCopy(nil)
			if err != nil {
				return errors.WithStack(err)
			}

			newKeypath := oldKeypath.Copy()
			copy(newKeypath[keypathLengthAsParent:keypathLengthAsParent+8], EncodeSliceIndex(newIdx))

			err = tx.tx.Set(newKeypath, valueBuf)
			if err != nil {
				return errors.Wrapf(err, "can't set keypath %v", newKeypath)
			}
			err = tx.tx.Delete(oldKeypath)
			if err != nil {
				return errors.Wrapf(err, "can't delete keypath %v", oldKeypath)
			}
		}
		iter.Close()
	}

	// Set new slice length
	if rng != nil && rootNodeType == NodeTypeSlice {
		newLen := length - rng.Size()
		encoded, err := encodeNode(NodeTypeSlice, ValueTypeInvalid, newLen, nil)
		if err != nil {
			return err
		}
		err = tx.tx.Set(relKeypath, encoded)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	// @@TODO: make sure that everywhere we're putting things into the Diff, we use tx.rmKeyPrefix(...)

	return nil
}

func (tx *DBNode) Diff() *Diff {
	return tx.diff
}

func (tx *DBNode) ResetDiff() {
	tx.diff = NewDiff()
}

func (tx *DBNode) CopyToMemory(keypath Keypath, rng *Range) (n Node, err error) {
	var valBytes []byte
	defer annotate(&err, "CopyToMemory (keypath: %v, range: %v, encoded value: %v)", keypath, rng, string(valBytes))

	if rng == nil {
		rng = tx.rng
	} else if rng != nil && tx.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return nil, ErrInvalidRange
	}

	keypath = tx.addKeyPrefix(tx.rootKeypath.Push(keypath))

	item, err := tx.tx.Get(keypath)
	if err != nil && errors.Cause(err) == badger.ErrKeyNotFound {
		return nil, types.Err404
	} else if err != nil {
		return nil, err
	}

	valBytes, err = item.ValueCopy(valBytes)
	if err != nil {
		return nil, err
	}

	rootNodeType, valueType, length, data, err := decodeNode(valBytes)
	if err != nil {
		return nil, errors.Wrapf(err, "(data: %v)", string(valBytes))
	}

	mNode := NewMemoryNode().(*MemoryNode)

	if rootNodeType == NodeTypeValue {
		v, err := decodeGoValue(rootNodeType, valueType, length, rng, data)
		if err != nil {
			return nil, err
		}
		err = mNode.Set(nil, nil, v)
		if err != nil {
			return nil, err
		}
		return mNode, nil
	}

	var startKeypath Keypath
	var endIdx uint64

	if len(tx.rmKeyPrefix(keypath)) > 0 {
		mNode.keypaths = append(mNode.keypaths, Keypath(nil))
	}
	mNode.nodeTypes[""] = rootNodeType

	if rootNodeType == NodeTypeMap {
		if rng != nil {
			return nil, ErrRangeOverNonSlice
		}
		if len(tx.rmKeyPrefix(keypath)) > 0 {
			startKeypath = append(keypath, KeypathSeparator[0])
		} else {
			startKeypath = keypath
		}

	} else if rootNodeType == NodeTypeSlice {
		if rng != nil {
			if !rng.ValidForLength(length) {
				return nil, ErrInvalidRange
			}

			mNode.sliceLengths[""] = int(rng.Size())

			var startIdx uint64
			startIdx, endIdx = rng.IndicesForLength(length)
			startKeypath = keypath.PushIndex(startIdx)
		} else {
			mNode.sliceLengths[""] = int(length)
			startKeypath = append(keypath, KeypathSeparator[0])
		}
	}

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	opts.Prefix = keypath
	iter := tx.tx.NewIterator(opts)
	defer iter.Close()

	var newKeypaths []Keypath
	var valBuf []byte

	for iter.Seek(startKeypath); iter.ValidForPrefix(startKeypath); iter.Next() {
		item := iter.Item()
		absKeypath := Keypath(item.KeyCopy(nil))
		relKeypath := tx.rmKeyPrefix(absKeypath).RelativeTo(tx.rmKeyPrefix(keypath))
		if len(relKeypath) == 0 {
			relKeypath = nil
		}

		// If we're ranging over a slice...
		if rng != nil {
			// And we've passed the end of the range, break
			idx := DecodeSliceIndex(relKeypath[:8])
			if idx >= endIdx {
				break
			}

			// Transpose its indices so that they start from 0
			if len(relKeypath) > 0 && rootNodeType == NodeTypeSlice {
				relKeypath = relKeypath.Copy()
				newIdx := uint64(int64(idx) - rng[0])
				copy(relKeypath[:8], EncodeSliceIndex(newIdx))
			}
		}

		err := item.Value(func(bs []byte) error {
			valBuf = bs
			return nil
		})
		if err != nil {
			return nil, err
		}

		nodeType, valueType, length, data, err := decodeNode(valBuf)
		if err != nil {
			return nil, err
		}

		// Only apply the passed range to the root value
		var thisRng *Range
		if len(relKeypath) == 0 {
			thisRng = rng
		}

		decoded, err := decodeGoValue(nodeType, valueType, length, thisRng, data)
		if err != nil {
			return nil, err
		}

		newKeypaths = append(newKeypaths, relKeypath)
		mNode.values[string(relKeypath)] = decoded
		mNode.nodeTypes[string(relKeypath)] = nodeType
		if nodeType == NodeTypeSlice {
			mNode.sliceLengths[string(relKeypath)] = int(length)
		}
	}
	mNode.addKeypaths(newKeypaths)
	mNode.diff = tx.diff.Copy()
	return mNode, nil
}

func (tx *DBNode) DebugPrint() {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	iter := tx.tx.NewIterator(opts)
	defer iter.Close()

	fmt.Println("- root keypath:", tx.rootKeypath)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		valBytes, err := item.ValueCopy(nil)
		if err != nil {
			panic(err)
		}

		nodeType, valueType, length, data, err := decodeNode(valBytes)
		if err != nil {
			panic(err)
		}

		val, err := decodeGoValue(nodeType, valueType, length, nil, data)
		if err != nil {
			panic(err)
		}

		if item.IsDeletedOrExpired() {
			fmt.Printf("  - %s %v %v %v %v (DELETED)\n", tx.rmKeyPrefix(Keypath(item.Key())), nodeType, valueType, length, val)
		} else {
			fmt.Printf("  - %s %v %v %v %v\n", tx.rmKeyPrefix(Keypath(item.Key())), nodeType, valueType, length, val)
		}
	}
}

func (t *DBTree) CopyVersion(dstVersion, srcVersion types.ID) error {
	return t.db.Update(func(tx *badger.Txn) error {
		stream := t.db.NewStream()
		stream.NumGo = 16
		stream.Prefix = append(srcVersion[:], ':')

		stream.Send = func(list *badgerpb.KVList) error {
			for _, kv := range list.Kv {
				newKey := kv.Key
				copy(newKey[:stateKeyPrefixLen], dstVersion[:])
				err := tx.Set(newKey, kv.Value)
				if err != nil {
					return err
				}
			}
			return nil
		}

		return stream.Orchestrate(context.TODO())
	})
}

func (n *DBNode) MarshalJSON() ([]byte, error) {
	v, _, err := n.Value(nil, nil)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func (t *DBTree) DebugPrint(keypathPrefix Keypath, rng *Range) ([]Keypath, []interface{}, error) {
	keypaths := make([]Keypath, 0)
	values := make([]interface{}, 0)

	err := t.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		iter := txn.NewIterator(opts)
		defer iter.Close()

		startKeypath := keypathPrefix
		var endKeypath Keypath
		if rng != nil {
			startKeypath = keypathPrefix.PushIndex(uint64(rng[0]))
			endKeypath = keypathPrefix.PushIndex(uint64(rng[1]))
		}

		for iter.Seek(startKeypath); iter.ValidForPrefix(keypathPrefix); iter.Next() {
			if endKeypath != nil && endKeypath.Equals(iter.Item().Key()) {
				break
			}
			keypaths = append(keypaths, Keypath(iter.Item().KeyCopy(nil)))
			encoded, err := iter.Item().ValueCopy(nil)
			if err != nil {
				return err
			}

			nodeType, valueType, length, data, err := decodeNode(encoded)
			if err != nil {
				return err
			}

			val, err := decodeGoValue(nodeType, valueType, length, rng, data)
			if err != nil {
				return err
			}
			values = append(values, val)
		}
		return nil
	})
	return keypaths, values, err
}

func (n *DBNode) Iterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	opts := badger.DefaultIteratorOptions
	opts.Reverse = false
	opts.PrefetchValues = prefetchValues
	opts.PrefetchSize = prefetchSize
	iter := n.tx.NewIterator(opts)
	return newIteratorFromBadgerIterator(iter, keypath, n)
}

func (n *DBNode) ChildIterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	dbIter := n.Iterator(keypath, prefetchValues, prefetchSize).(*dbIterator)
	return &dbChildIterator{
		dbIterator:              dbIter,
		strippedAbsKeypathParts: n.rmKeyPrefix(dbIter.rootKeypath).NumParts(),
	}
}

func (n *DBNode) DepthFirstIterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	opts := badger.DefaultIteratorOptions
	opts.Reverse = true
	opts.PrefetchValues = prefetchValues
	opts.PrefetchSize = prefetchSize
	iter := n.tx.NewIterator(opts)

	rootKeypath := n.addKeyPrefix(n.rootKeypath.Push(keypath))
	scanPrefix := rootKeypath
	if len(scanPrefix) != len(n.keyPrefix) {
		scanPrefix = append(scanPrefix, KeypathSeparator[0])
	}

	iter.Seek(append(scanPrefix, byte(0xff)))

	return &dbDepthFirstIterator{
		iter:        iter,
		rootKeypath: rootKeypath,
		scanPrefix:  scanPrefix,
		tx:          n.tx,
		rootNode:    n,
		iterNode:    &DBNode{tx: n.tx},
	}
}

type Indexer interface {
	IndexKeyForNode(node Node) (Keypath, error)
}

func prettyJSON(x interface{}) string {
	j, _ := json.MarshalIndent(x, "", "    ")
	return string(j)
}

func (t *DBTree) BuildIndex(version *types.ID, keypath Keypath, node Node, indexName Keypath, indexer Indexer) (err error) {
	defer annotate(&err, "BuildIndex")

	// @@TODO: ensure NodeType is map or slice
	// @@TODO: don't use a map[][] to count children, put it in Badger
	index := t.IndexAtVersion(version, keypath, indexName, true)
	defer index.Close()

	iter := node.ChildIterator(nil, true, 10)
	defer iter.Close()

	rootNodeType, _, _, err := node.NodeInfo(nil)
	if err != nil {
		t.Error(err)
		return err
	}

	children := make(map[string]map[string]struct{})
	for iter.Rewind(); iter.Valid(); iter.Next() {
		childNode := iter.Node()
		relKeypath := childNode.Keypath().RelativeTo(node.Keypath())

		indexKey, err := indexer.IndexKeyForNode(childNode)
		if err != nil {
			return err
		} else if indexKey == nil {
			continue
		}

		if _, exists := children[string(indexKey)]; !exists {
			children[string(indexKey)] = make(map[string]struct{})
		}
		children[string(indexKey)][string(relKeypath.Part(0))] = struct{}{}

		if rootNodeType == NodeTypeSlice {
			// If it's a slice, we have to renumber its children
			newIdx := uint64(len(children[string(indexKey)])) - 1
			_, rest := relKeypath.Shift()
			relKeypath = rest.Unshift(EncodeSliceIndex(newIdx))

		} else if rootNodeType == NodeTypeMap {
			// If it's a map, we have to replace the root key with the indexKey
			_, rest := relKeypath.Shift()
			relKeypath = rest.Unshift(indexKey)
		}

		err = index.Set(relKeypath, nil, childNode)
		if err != nil {
			t.Error(err)
			return err
		}
	}

	// Set the index's node types to the type of the original keypath being indexed
	for indexKey, child := range children {
		encoded, err := encodeNode(rootNodeType, 0, uint64(len(child)), nil)
		if err != nil {
			return err
		}

		err = index.tx.Set(index.addKeyPrefix(Keypath(indexKey)), encoded)
		if err != nil {
			return err
		}
	}

	// Set the root value of the index to a NodeTypeMap
	encoded, err := encodeNode(NodeTypeMap, 0, 0, nil)
	if err != nil {
		return err
	}
	err = index.tx.Set(index.addKeyPrefix(nil), encoded)
	if err != nil {
		return err
	}
	return index.Save()
}

func encodeNode(nodeType NodeType, valueType ValueType, length uint64, value interface{}) ([]byte, error) {
	switch nodeType {
	case NodeTypeMap:
		encoded := make([]byte, 9)
		encoded[0] = 'm'
		copy(encoded[1:], EncodeSliceLen(length))
		return encoded, nil

	case NodeTypeSlice:
		encoded := make([]byte, 9)
		encoded[0] = 's'
		copy(encoded[1:], EncodeSliceLen(length))
		return encoded, nil

	case NodeTypeValue:

		switch valueType {
		case ValueTypeBool:
			if value == true {
				return []byte{'v', 'b', 1}, nil
			} else {
				return []byte{'v', 'b', 0}, nil
			}

		case ValueTypeUint:
			v := value.(uint64)
			encoded := make([]byte, 10)
			encoded[0] = 'v'
			encoded[1] = 'u'
			binary.LittleEndian.PutUint64(encoded[2:10], v)
			return encoded, nil

		case ValueTypeInt:
			v := value.(int64)
			encodedBuf := &bytes.Buffer{}
			_, err := encodedBuf.Write([]byte("vi"))
			if err != nil {
				return nil, err
			}
			err = binary.Write(encodedBuf, binary.LittleEndian, v)
			if err != nil {
				return nil, err
			}
			return encodedBuf.Bytes(), nil

		case ValueTypeFloat:
			v := value.(float64)
			encodedBuf := &bytes.Buffer{}
			_, err := encodedBuf.Write([]byte("vf"))
			if err != nil {
				return nil, err
			}
			err = binary.Write(encodedBuf, binary.LittleEndian, v)
			if err != nil {
				return nil, err
			}
			return encodedBuf.Bytes(), nil

		case ValueTypeString:
			v := value.(string)
			encoded := make([]byte, 2+len(v))
			encoded[0] = 'v'
			encoded[1] = 's'
			copy(encoded[2:], []byte(v))
			return encoded, nil

		case ValueTypeNil:
			encoded := []byte("v0")
			return encoded, nil

		default:
			return nil, errors.WithStack(ErrNodeEncoding)
		}
	default:
		return nil, errors.WithStack(ErrNodeEncoding)
	}
}

func encodeGoValue(nodeValue interface{}) ([]byte, error) {
	switch nv := nodeValue.(type) {
	case map[string]interface{}:
		return encodeNode(NodeTypeMap, 0, uint64(len(nv)), nil)
	case []interface{}:
		return encodeNode(NodeTypeSlice, 0, uint64(len(nv)), nil)
	case bool:
		return encodeNode(NodeTypeValue, ValueTypeBool, 0, nv)
	case uint64:
		return encodeNode(NodeTypeValue, ValueTypeUint, 0, nv)
	case int64:
		return encodeNode(NodeTypeValue, ValueTypeInt, 0, nv)
	case float64:
		return encodeNode(NodeTypeValue, ValueTypeFloat, 0, nv)
	case string:
		return encodeNode(NodeTypeValue, ValueTypeString, 0, nv)
	case nil:
		return encodeNode(NodeTypeValue, ValueTypeNil, 0, nv)
	default:
		return nil, errors.Errorf("cannot encode Go value of type (%T)", nodeValue)
	}
}

func decodeNode(val []byte) (NodeType, ValueType, uint64, []byte, error) {
	switch val[0] {
	case 'm':
		length := DecodeSliceLen(val[1:])
		return NodeTypeMap, ValueTypeInvalid, length, nil, nil
	case 's':
		length := DecodeSliceLen(val[1:])
		return NodeTypeSlice, ValueTypeInvalid, length, nil, nil
	case 'v':
		switch val[1] {
		case 'b':
			return NodeTypeValue, ValueTypeBool, 0, val[2:], nil
		case 'u':
			return NodeTypeValue, ValueTypeUint, 0, val[2:], nil
		case 'i':
			return NodeTypeValue, ValueTypeInt, 0, val[2:], nil
		case 'f':
			return NodeTypeValue, ValueTypeFloat, 0, val[2:], nil
		case 's':
			return NodeTypeValue, ValueTypeString, uint64(len(val[2:])), val[2:], nil
		case '0':
			return NodeTypeValue, ValueTypeNil, 0, nil, nil
		default:
			return NodeTypeInvalid, ValueTypeInvalid, 0, nil, errors.WithStack(ErrNodeEncoding)
		}
	default:
		return NodeTypeInvalid, ValueTypeInvalid, 0, nil, errors.WithStack(ErrNodeEncoding)
	}
}

func decodeGoValue(nodeType NodeType, valueType ValueType, length uint64, rng *Range, data []byte) (interface{}, error) {
	switch nodeType {
	case NodeTypeMap:
		if rng != nil {
			if !rng.ValidForLength(length) {
				return nil, errors.WithStack(ErrInvalidRange)
			}
			length = rng.Size()
		}
		return make(map[string]interface{}), nil

	case NodeTypeSlice:
		if rng != nil {
			if !rng.ValidForLength(length) {
				return nil, errors.WithStack(ErrInvalidRange)
			}
			length = rng.Size()
		}
		return make([]interface{}, length), nil

	case NodeTypeValue:
		switch valueType {
		case ValueTypeBool:
			if rng != nil {
				return nil, errors.WithStack(ErrRangeOverNonSlice)
			}
			if data[0] == byte(1) {
				return true, nil
			} else {
				return false, nil
			}

		case ValueTypeUint:
			if rng != nil {
				return nil, errors.WithStack(ErrRangeOverNonSlice)
			}
			return binary.LittleEndian.Uint64(data), nil

		case ValueTypeInt:
			if rng != nil {
				return nil, errors.WithStack(ErrRangeOverNonSlice)
			}
			var i int64
			buf := bytes.NewReader(data)
			err := binary.Read(buf, binary.LittleEndian, &i)
			if err != nil {
				return nil, errors.Wrapf(err, "could not read int64")
			}
			return i, nil

		case ValueTypeFloat:
			if rng != nil {
				return nil, errors.WithStack(ErrRangeOverNonSlice)
			}
			var f float64
			buf := bytes.NewReader(data)
			err := binary.Read(buf, binary.LittleEndian, &f)
			if err != nil {
				return nil, errors.Wrapf(err, "could not read float64")
			}
			return f, nil

		case ValueTypeString:
			if rng != nil {
				if !rng.ValidForLength(length) {
					return nil, errors.WithStack(ErrInvalidRange)
				}
				startIdx, endIdx := rng.IndicesForLength(length)
				return string(data[startIdx:endIdx]), nil
			}
			return string(data), nil

		case ValueTypeNil:
			return nil, nil

		default:
			return nil, errors.WithStack(ErrNodeEncoding)
		}

	default:
		return nil, errors.WithStack(ErrNodeEncoding)
	}
}
