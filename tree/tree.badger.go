package tree

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/dgraph-io/badger/v2"
	badgerpb "github.com/dgraph-io/badger/v2/pb"
	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/types"
)

type DBTree struct {
	db       *badger.DB
	filename string
}

func NewDBTree(dbFilename string) (*DBTree, error) {
	opts := badger.DefaultOptions(dbFilename)
	opts.Logger = nil

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &DBTree{db, dbFilename}, nil
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
	return bytes.Join([][]byte{[]byte("i"), version[:], keypath, indexName}, []byte(":"))
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
	return tx.tx.Commit()
}

func (tx *DBNode) Keypath() Keypath {
	return tx.rootKeypath
}

func (tx *DBNode) AtKeypath(keypath Keypath, rng *Range) Node {
	return &DBNode{tx: tx.tx, rootKeypath: tx.rootKeypath.Push(keypath), rng: tx.rng, keyPrefix: tx.keyPrefix, diff: tx.diff}
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

func (tx *DBNode) NodeInfo() (NodeType, ValueType, uint64, error) {
	item, err := tx.tx.Get(tx.addKeyPrefix(tx.rootKeypath))
	if err == badger.ErrKeyNotFound {
		return 0, 0, 0, errors.WithStack(types.Err404)
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
		return nil, false, errors.WithStack(ErrInvalidRange)
	}

	keypathPrefix = tx.addKeyPrefix(tx.rootKeypath.Push(keypathPrefix))
	goValues := make(map[string]interface{})

	item, err := tx.tx.Get(keypathPrefix)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			err = errors.Wrapf(types.Err404, "(keypath: %v)", keypathPrefix)
		}
		return nil, false, errors.WithStack(err)
	}

	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, false, errors.WithStack(err)
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
	var endKeypathPrefix Keypath

	if rootNodeType == NodeTypeMap {
		if rng != nil {
			return nil, false, errors.WithStack(ErrRangeOverNonSlice)
		}
		startKeypath = keypathPrefix

	} else if rootNodeType == NodeTypeSlice {
		if rng != nil {
			if !rng.ValidForLength(length) {
				return nil, false, errors.WithStack(ErrInvalidRange)
			}
			startIdx, endIdx := rng.IndicesForLength(length)
			startKeypath = keypathPrefix.PushIndex(startIdx)
			endKeypathPrefix = keypathPrefix.PushIndex(endIdx)
		} else {
			startKeypath = keypathPrefix
		}
	}

	var valueBuf []byte
	var validPrefix = keypathPrefix
	for iter.Seek(startKeypath); iter.ValidForPrefix(validPrefix); iter.Next() {
		item := iter.Item()
		absKeypath := Keypath(item.Key())

		// If we're ranging over a slice, and we find the first keypath of the final element,
		// swap the keypath prefix the iterator is checking against (which WAS the root node
		// keypath) with the keypath of the final element.  The iterator will then stop after
		// the final keypath for that final element.
		if rng != nil && absKeypath.Equals(endKeypathPrefix) {
			validPrefix = append(endKeypathPrefix, KeypathSeparator[0])
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
				return nil, false, errors.WithStack(err)
			}
			defer annotate(&err, "(valueBuf length: %v, valueBuf: %v)", len(valueBuf), string(valueBuf))

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
				return nil, false, errors.WithStack(err)
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
				panic(fmt.Sprintf("bad parent type: [%v] (%T) %v  //  (%T) %v", relKeypath, parent, parent, val, val))
			}
		}
	}

	root, exists := goValues[""]
	return root, exists, nil
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

func (tx *DBNode) Set(absKeypath Keypath, rng *Range, val interface{}) error {
	if rng == nil {
		rng = tx.rng
	} else if rng != nil && tx.rng != nil {
		panic("unsupported")
	}

	absKeypath = tx.rootKeypath.Push(absKeypath)

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
	return tx.tx.Set(tx.addKeyPrefix(tx.rootKeypath.Push(absKeypath)), newVal)
}

func (tx *DBNode) setRangeSlice(absKeypath Keypath, rng *Range, encodedVal []byte, spliceVal []interface{}) error {
	nodeType, _, oldLen, _, err := decodeNode(encodedVal)
	if err != nil {
		return err
	} else if nodeType != NodeTypeSlice {
		return errors.WithStack(ErrRangeOverNonSlice)
	} else if !rng.ValidForLength(oldLen) {
		return errors.WithStack(ErrInvalidRange)
	}

	absKeypath = tx.addKeyPrefix(tx.rootKeypath.Push(absKeypath))

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
	if newLen != oldLen {
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
				return errors.WithStack(err)
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
			encoded := make([]byte, 9)
			encoded[0] = 's'
			copy(encoded[1:], EncodeSliceLen(newLen))

			err := tx.tx.Set(absKeypath, encoded)
			if err != nil {
				return errors.WithStack(err)
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
	err := tx.Delete(absKeypath, nil)
	if err != nil {
		return err
	}

	absKeypath = tx.rootKeypath.Push(absKeypath)

	// Set value types for intermediate keypaths in case they don't exist
	partialKeypath, _ := absKeypath.Pop()
	partialKeypath = tx.addKeyPrefix(partialKeypath)
	for {
		item, err := tx.tx.Get(partialKeypath)
		if err == badger.ErrKeyNotFound {
			err = tx.tx.Set(partialKeypath, []byte("m"))
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			val, err := item.ValueCopy(nil)
			if val[0] == 'v' {
				err = tx.tx.Set(partialKeypath, []byte("m"))
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

	// @@TODO: handle tree.Node values
	err = walkGoValue(value, func(nodeKeypath Keypath, nodeValue interface{}) error {
		absNodeKeypath := absKeypath
		if len(nodeKeypath) != 0 {
			absNodeKeypath = absKeypath.Push(nodeKeypath)
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

func (tx *DBNode) Delete(keypathPrefix Keypath, rng *Range) error {
	if rng == nil {
		rng = tx.rng
	} else if rng != nil && tx.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return errors.WithStack(ErrInvalidRange)
	}

	keypathPrefix = tx.addKeyPrefix(tx.rootKeypath.Push(keypathPrefix))

	item, err := tx.tx.Get(keypathPrefix)
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

			tx.diff.Remove(tx.rmKeyPrefix(keypathPrefix))
			return tx.Set(keypathPrefix, nil, s)
		}

		tx.diff.Remove(tx.rmKeyPrefix(keypathPrefix))
		return tx.tx.Delete(keypathPrefix)
	}

	var startIdx, endIdx uint64
	var startKeypath Keypath
	var endKeypathPrefix Keypath

	if rootNodeType == NodeTypeMap {
		if rng != nil {
			return errors.WithStack(ErrRangeOverNonSlice)
		}
		startKeypath = keypathPrefix

	} else if rootNodeType == NodeTypeSlice {
		if rng != nil {
			if !rng.ValidForLength(length) {
				return errors.WithStack(ErrInvalidRange)
			}
			startIdx, endIdx = rng.IndicesForLength(length)
		}
		startKeypath = keypathPrefix.PushIndex(startIdx)
		endKeypathPrefix = keypathPrefix.PushIndex(endIdx)
	}

	// Delete items
	{
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false

		iter := tx.tx.NewIterator(opts)
		defer iter.Close()

		var validPrefix = startKeypath
		for iter.Seek(startKeypath); iter.ValidForPrefix(validPrefix); iter.Next() {
			absKeypath := Keypath(iter.Item().Key())

			// If we're ranging over a slice, and we find the first keypath of the final element,
			// swap the keypath prefix the iterator is checking against (which WAS the root node
			// keypath) with the keypath of the final element.  The iterator will then stop after
			// the final keypath for that final element.
			if rng != nil && absKeypath.Equals(endKeypathPrefix) {
				validPrefix = append(endKeypathPrefix, KeypathSeparator[0])
			}

			err := tx.tx.Delete(absKeypath)
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

		startKeypath := keypathPrefix.PushIndex(endIdx)
		endKeypathPrefix := keypathPrefix.PushIndex(length)
		validPrefix := keypathPrefix
		delta := -int64(rng.Size())
		keypathLengthAsParent := len(tx.keyPrefix) + tx.rmKeyPrefix(keypathPrefix).LengthAsParent()

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
		encoded := make([]byte, 9)
		encoded[0] = 's'
		copy(encoded[1:], EncodeSliceLen(newLen))

		err := tx.tx.Set(keypathPrefix, encoded)
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

	mNode.keypaths = append(mNode.keypaths, Keypath(nil))
	mNode.nodeTypes[""] = rootNodeType

	if rootNodeType == NodeTypeMap {
		if rng != nil {
			return nil, ErrRangeOverNonSlice
		}
		startKeypath = append(keypath, KeypathSeparator[0])

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

	validPrefix := append(keypath, KeypathSeparator[0])

	for iter.Seek(startKeypath); iter.ValidForPrefix(validPrefix); iter.Next() {
		item := iter.Item()
		absKeypath := Keypath(item.KeyCopy(nil))
		relKeypath := tx.rmKeyPrefix(absKeypath).RelativeTo(tx.rmKeyPrefix(keypath))

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

		fmt.Printf("  - %v %v %v %v %v\n", Keypath(item.Key()), nodeType, valueType, length, val)
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

func (tx *DBNode) DepthFirstIterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	opts := badger.DefaultIteratorOptions
	opts.Reverse = true
	opts.PrefetchValues = prefetchValues
	opts.PrefetchSize = prefetchSize
	iter := tx.tx.NewIterator(opts)

	absKeypath := tx.addKeyPrefix(tx.rootKeypath.Push(keypath))
	scanPrefix := absKeypath
	if len(scanPrefix) != len(tx.keyPrefix) {
		scanPrefix = append(scanPrefix, KeypathSeparator[0])
	}

	iter.Seek(append(scanPrefix, byte(0xff)))

	return &dbDepthFirstIterator{
		iter:       iter,
		absKeypath: absKeypath,
		scanPrefix: scanPrefix,
		tx:         tx,
		iterNode: &dbDepthFirstIteratorItem{
			DBNode:     &DBNode{tx: tx.tx},
			badgerItem: nil,
		},
	}
}

type dbDepthFirstIterator struct {
	iter       *badger.Iterator
	absKeypath Keypath
	scanPrefix Keypath
	tx         *DBNode
	iterNode   *dbDepthFirstIteratorItem
	done       bool
}

// Ensure that dbDepthFirstIterator implements the Iterator interface
var _ Iterator = (*dbDepthFirstIterator)(nil)

// Ensure that dbDepthFirstIteratorItem implements the Node interface
var _ Node = (*dbDepthFirstIteratorItem)(nil)

type dbDepthFirstIteratorItem struct {
	badgerItem *badger.Item
	*DBNode
}

func (iter *dbDepthFirstIterator) Next() Node {
	iter.iter.Next()
	if !iter.iter.ValidForPrefix(iter.scanPrefix) {
		if !iter.done {
			iter.done = true

			item, err := iter.tx.tx.Get(iter.absKeypath)
			if err != nil {
				// @@TODO: add an `err` field to the iterator?
				return nil
			}

			iter.iterNode.rootKeypath = item.KeyCopy(iter.iterNode.rootKeypath)
			iter.iterNode.badgerItem = item
			return iter.iterNode

		} else {
			return nil
		}
	}

	item := iter.iter.Item()
	iter.iterNode.rootKeypath = item.KeyCopy(iter.iterNode.rootKeypath)
	iter.iterNode.badgerItem = item
	return iter.iterNode
}

func (iter *dbDepthFirstIterator) Close() {
	iter.iter.Close()
}

type Indexer interface {
	IndexKeyForNode(node Node) (Keypath, error)
}

func (t *DBTree) BuildIndex(version *types.ID, node *DBNode, indexName Keypath, indexer Indexer) (err error) {
	defer annotate(&err, "BuildIndex")

	// @@TODO: ensure NodeType is map or slice
	// @@TODO: don't use a map[][] to count children, put it in Badger
	index := t.IndexAtVersion(version, node.Keypath(), indexName, true)
	defer index.Close()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchSize = 10
	iter := node.tx.NewIterator(opts)
	defer iter.Close()

	startKeypath := append(node.addKeyPrefix(node.Keypath()), KeypathSeparator[0])

	iterNode := &dbNodeWithIterator{iter: iter}

	rootNodeType, _, _, err := node.NodeInfo()
	if err != nil {
		return err
	}

	var indexKey Keypath
	children := make(map[string]map[string]struct{})
	for iter.Seek(startKeypath); iter.ValidForPrefix(startKeypath); iter.Next() {
		item := iter.Item()
		absKeypath := Keypath(item.KeyCopy(nil))
		relKeypath := node.rmKeyPrefix(absKeypath).RelativeTo(node.Keypath())

		newRelKeypath := relKeypath.Copy()
		if absKeypath.NumParts() == node.Keypath().NumParts()+1 {
			iterNode.DBNode = node.AtKeypath(relKeypath, nil).(*DBNode)

			var err error
			indexKey, err = indexer.IndexKeyForNode(iterNode)
			if err != nil {
				return err
			}

			iter.Seek(absKeypath)

			if _, exists := children[string(indexKey)]; !exists {
				children[string(indexKey)] = make(map[string]struct{})
			}
			children[string(indexKey)][string(relKeypath.Part(0))] = struct{}{}
		}

		// If it's a slice, we have to renumber its children
		if rootNodeType == NodeTypeSlice {
			newIdx := uint64(len(children[string(indexKey)])) - 1
			_, rest := newRelKeypath.Shift()
			newRelKeypath = rest.Unshift(EncodeSliceIndex(newIdx))
		}

		valBytes, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		err = index.tx.Set(index.addKeyPrefix(indexKey.Push(newRelKeypath)), valBytes)
		if err != nil {
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

type dbNodeWithIterator struct {
	*DBNode
	iter *badger.Iterator
}

func (n *dbNodeWithIterator) Value(keypath Keypath, rng *Range) (interface{}, bool, error) {
	return n.DBNode.value(keypath, rng, n.iter)
}

func encodeNode(nodeType NodeType, valueType ValueType, length uint64, value interface{}) ([]byte, error) {
	switch nodeType {
	case NodeTypeMap:
		return []byte("m"), nil

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
		return encodeNode(NodeTypeMap, 0, 0, nil)
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
		return NodeTypeMap, ValueTypeInvalid, 0, nil, nil
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
			return nil, errors.WithStack(ErrRangeOverNonSlice)
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
