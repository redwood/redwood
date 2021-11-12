package state

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/brynbellomy/go-structomancer"
	"github.com/dgraph-io/badger/v2"
	badgerpb "github.com/dgraph-io/badger/v2/pb"

	"redwood.dev/errors"
	"redwood.dev/log"
)

type DBTree struct {
	db       *badger.DB
	filename string
	log.Logger
}

type EncryptionConfig struct {
	Key                 []byte        `json:"key"`
	KeyRotationInterval time.Duration `json:"rotationInterval"`
}

func NewDBTree(badgerOpts badger.Options) (*DBTree, error) {
	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, err
	}
	return &DBTree{db, badgerOpts.Dir, log.NewLogger("db tree")}, nil
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

func (t *DBTree) State(mutable bool) *DBNode {
	var diff *Diff
	if mutable {
		diff = NewDiff()
	}
	return &DBNode{
		tx:        t.db.NewTransaction(mutable),
		keyPrefix: KeypathSeparator,
		diff:      diff,
		activeIterator: &activeIterator{
			mutable: mutable,
		},
	}
}

type VersionedDBTree struct {
	db       *badger.DB
	filename string
	log.Logger
}

func NewVersionedDBTree(badgerOpts badger.Options) (*VersionedDBTree, error) {
	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, err
	}
	return &VersionedDBTree{db, badgerOpts.Dir, log.NewLogger("db tree")}, nil
}

func (t *VersionedDBTree) Close() error {
	return t.db.Close()
}

func (t *VersionedDBTree) DeleteDB() error {
	err := t.Close()
	if err != nil {
		return err
	}
	return os.RemoveAll(t.filename)
}

var stateKeyPrefixLen = len(Version{}) + 1

func (t *VersionedDBTree) makeStateKeyPrefix(version Version) []byte {
	// <version>:
	keyPrefix := make([]byte, stateKeyPrefixLen)
	copy(keyPrefix, version.Bytes())
	keyPrefix[len(keyPrefix)-1] = ':'
	return keyPrefix
}

func (t *VersionedDBTree) makeIndexKeyPrefix(version Version, keypath Keypath, indexName Keypath) []byte {
	// i:<version>:<keypath>:<indexName>:
	// i:deadbeef19482:foo/messages:author:
	return bytes.Join([][]byte{[]byte("i"), version[:], keypath, indexName, []byte{}}, []byte(":"))
}

func (t *VersionedDBTree) StateAtVersion(version *Version, mutable bool) *DBNode {
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
		activeIterator: &activeIterator{
			mutable: mutable,
		},
	}
}

func (t *VersionedDBTree) IndexAtVersion(version *Version, keypath Keypath, indexName Keypath, mutable bool) *DBNode {
	if version == nil {
		version = &CurrentVersion
	}
	return &DBNode{
		tx:        t.db.NewTransaction(mutable),
		keyPrefix: t.makeIndexKeyPrefix(*version, keypath, indexName),
		activeIterator: &activeIterator{
			mutable: mutable,
		},
	}
}

type DBNode struct {
	tx             *badger.Txn
	diff           *Diff
	keyPrefix      []byte
	rootKeypath    Keypath
	rng            *Range
	activeIterator *activeIterator
}

type activeIterator struct {
	mutable bool
	iter    Iterator
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

func (tx *DBNode) addKeyPrefix(keypath Keypath) Keypath {
	return append(tx.keyPrefix, keypath...)
}

func (tx *DBNode) rmKeyPrefix(keypath Keypath) Keypath {
	return keypath[len(tx.keyPrefix):]
}

func (tx *DBNode) Keypath() Keypath {
	return tx.rootKeypath
}

func (n *DBNode) NodeAt(keypath Keypath, rng *Range) Node {
	if keypath.Equals(KeypathSeparator) {
		keypath = nil
	}
	return &DBNode{
		tx:             n.tx,
		rootKeypath:    n.rootKeypath.Push(keypath),
		rng:            n.rng,
		keyPrefix:      n.keyPrefix,
		diff:           n.diff,
		activeIterator: n.activeIterator,
	}
}

func (n *DBNode) ParentNodeFor(keypath Keypath) (Node, Keypath) {
	return n, keypath
}

func (tx *DBNode) Subkeys() []Keypath {
	opts := badger.DefaultIteratorOptions
	iter := tx.tx.NewIterator(opts)
	defer iter.Close()

	startKeypath := append(tx.addKeyPrefix(tx.rootKeypath), KeypathSeparator[0])

	var keypaths []Keypath
	keypathsMap := make(map[string]struct{})
	for iter.Seek(startKeypath); iter.ValidForPrefix(startKeypath); iter.Next() {
		item := iter.Item()
		absKeypath := Keypath(item.Key()).Copy()
		subkey := tx.rmKeyPrefix(absKeypath).RelativeTo(tx.rootKeypath).Part(0).Copy()
		_, exists := keypathsMap[string(subkey)]
		if !exists && len(subkey) > 0 {
			keypaths = append(keypaths, subkey)
			keypathsMap[string(subkey)] = struct{}{}
		}
	}
	return keypaths
}

func (tx *DBNode) NumSubkeys() uint64 {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	iter := tx.tx.NewIterator(opts)
	defer iter.Close()

	startKeypath := append(tx.addKeyPrefix(tx.rootKeypath), KeypathSeparator[0])

	var i uint64
	for iter.Seek(startKeypath); iter.ValidForPrefix(startKeypath); iter.Next() {
		item := iter.Item()
		absKeypath := Keypath(item.Key())
		subkey := tx.rmKeyPrefix(absKeypath).RelativeTo(tx.rootKeypath).Part(0)
		if len(subkey) > 0 {
			i++
		}
	}
	return i
}

func (n *DBNode) NodeInfo(keypath Keypath) (NodeType, ValueType, uint64, error) {
	return n.nodeInfo(n.rootKeypath.Push(keypath))
}

func (n *DBNode) nodeInfo(absKeypath Keypath) (NodeType, ValueType, uint64, error) {
	item, err := n.tx.Get(n.addKeyPrefix(absKeypath))
	if err == badger.ErrKeyNotFound {
		return 0, 0, 0, errors.Wrap(errors.Err404, n.addKeyPrefix(absKeypath).String())
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

func (n *DBNode) Exists(relKeypath Keypath) (bool, error) {
	return n.exists(n.rootKeypath.Push(relKeypath))
}

func (n *DBNode) exists(absKeypath Keypath) (bool, error) {
	_, err := n.tx.Get(n.addKeyPrefix(absKeypath))
	if err == badger.ErrKeyNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (tx *DBNode) Value(relKeypath Keypath, rng *Range) (_ interface{}, _ bool, err error) {
	defer errors.Annotate(&err, "Value (keypath: %v, range: %v)", relKeypath, rng)

	if rng == nil {
		rng = tx.rng
	} else if rng != nil && tx.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return nil, false, ErrInvalidRange
	}

	item, err := tx.tx.Get(tx.addKeyPrefix(tx.rootKeypath.Push(relKeypath)))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, false, nil
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
	defer errors.Annotate(&err, "(root nodeType: %v, root valueType: %v, root length: %v, root data: %v)", rootNodeType, valueType, length, string(data))

	if rootNodeType == NodeTypeValue {
		v, err := decodeGoValue(rootNodeType, valueType, length, rng, data)
		if err != nil {
			return nil, false, err
		}
		return v, true, nil
	}

	if rng != nil && !rng.ValidForLength(length) {
		return nil, false, ErrInvalidRange
	}

	goParents := make(map[string]interface{})
	var startIdx uint64

	if rootNodeType == NodeTypeMap {
		if rng != nil {
			goParents[""] = make(map[string]interface{}, rng.Length())
		} else {
			goParents[""] = make(map[string]interface{}, length)
		}
	} else if rootNodeType == NodeTypeSlice {
		if rng != nil {
			goParents[""] = make([]interface{}, rng.Length())
			startIdx, _ = rng.IndicesForLength(length)
		} else {
			goParents[""] = make([]interface{}, length)
		}
	}

	var valueBuf []byte
	absKeypath := tx.Keypath().Push(relKeypath)
	err = tx.scanChildrenForward(rootNodeType, absKeypath, rng, length, true, func(absChildKeypath Keypath, item *badger.Item) error {
		relKeypath := absChildKeypath.RelativeTo(absKeypath)

		// If we're ranging over a slice, transpose its indices to start from 0.
		if rootNodeType == NodeTypeSlice && rng != nil {
			newAbsKeypath := renumberSliceIndexKeypath(absKeypath, absChildKeypath, -int64(startIdx))
			relKeypath = newAbsKeypath.RelativeTo(absKeypath)
		}

		// Decode the value from the DB into a Go value
		var val interface{}
		{
			valueBuf, err = item.ValueCopy(valueBuf)
			if err != nil {
				return err
			}

			nodeType, valueType, length, data, err := decodeNode(valueBuf)
			if err != nil {
				return err
			}

			val, err = decodeGoValue(nodeType, valueType, length, nil, data)
			if err != nil {
				return err
			}

			// If this node is a map or slice, store it in `goParents` so that we can find it later
			// and put each of its children inside of it
			switch val.(type) {
			case map[string]interface{}, []interface{}:
				goParents[string(relKeypath)] = val
			}
		}

		// Insert the decoded value into its parent
		{
			parentKeypath, key := relKeypath.Pop()
			if len(key) == 0 {
				// This would indicate that we have somehow hit the root element, which should be impossible
				panic("should never happen")
			}

			parent := goParents[string(parentKeypath)]
			switch p := parent.(type) {
			case map[string]interface{}:
				p[string(key)] = val
			case []interface{}:
				p[DecodeSliceIndex(key)] = val
			default:
				panic(fmt.Sprintf("bad parent type: setting [%v].  %v is (%T) == %v", relKeypath, parentKeypath, parent, parent))
			}
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	root, exists := goParents[""]
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
	u, isUint := val.(uint64)
	if isUint {
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
	i, isInt := val.(int64)
	if isInt {
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

func (tx *DBNode) BoolValue(keypath Keypath) (bool, bool, error) {
	// @@TODO: maybe optimize the case where the keypath holds a complex object
	// that .Value takes a while to decode by first checking the value's type?
	val, exists, err := tx.Value(keypath, nil)
	if err != nil {
		return false, false, err
	} else if !exists {
		return false, false, nil
	}
	b, isBool := val.(bool)
	if isBool {
		return b, true, nil
	}
	return false, false, nil
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

func (tx *DBNode) BytesValue(keypath Keypath) ([]byte, bool, error) {
	// @@TODO: maybe optimize the case where the keypath holds a complex object
	// that .Value takes a while to decode by first checking the value's type?
	val, exists, err := tx.Value(keypath, nil)
	if err != nil {
		return nil, false, err
	} else if !exists {
		return nil, false, nil
	}
	bs, isBytes := val.([]byte)
	if !isBytes {
		return nil, false, nil
	}
	return bs, true, nil
}

func (tx *DBNode) MapValue(keypath Keypath) (map[string]interface{}, bool, error) {
	// @@TODO: maybe optimize the case where the keypath holds a complex object
	// that .Value takes a while to decode by first checking the value's type?
	val, exists, err := tx.Value(keypath, nil)
	if err != nil {
		return nil, false, err
	} else if !exists {
		return nil, false, nil
	}
	m, isMap := val.(map[string]interface{})
	if !isMap {
		return nil, false, nil
	}
	return m, true, nil
}

func (tx *DBNode) SliceValue(keypath Keypath) ([]interface{}, bool, error) {
	// @@TODO: maybe optimize the case where the keypath holds a complex object
	// that .Value takes a while to decode by first checking the value's type?
	val, exists, err := tx.Value(keypath, nil)
	if err != nil {
		return nil, false, err
	} else if !exists {
		return nil, false, nil
	}
	s, isSlice := val.([]interface{})
	if !isSlice {
		return nil, false, nil
	}
	return s, true, nil
}

func makeScanError(node *DBNode, dest interface{}) error {
	nodeType, valueType, _, _ := node.NodeInfo(nil)
	return errors.Wrapf(ErrWrongType, "cannot scan a (%s:%s) into a %T (keypath: %v)", nodeType, valueType, dest, node.Keypath())
}

func (node *DBNode) Scan(into interface{}) error {
	return node.scan(reflect.ValueOf(into).Elem())
}

func (node *DBNode) scan(rval reflect.Value) error {
	if rval.Type().Implements(stateBytesUnmarshalerType) {
		encoded, is, err := node.BytesValue(nil)
		if err != nil {
			return err
		} else if !is {
			return nil // @@TODO: ?
		}
		if rval.Type().Kind() == reflect.Ptr {
			if rval.CanAddr() {
				rval.Set(reflect.New(rval.Type().Elem()))
			} else {
				rval.Elem().Set(reflect.New(rval.Type().Elem()).Elem())
			}
		} else {
			rval.Set(reflect.New(rval.Type()).Elem())
		}
		rval.Interface().(StateBytesUnmarshaler).UnmarshalStateBytes(encoded)
		return nil
	}

	switch rval.Type().Kind() {
	case reflect.Ptr:
		rvalType := rval.Type()
		rvalElemType := rvalType.Elem()
		if rval.CanSet() {
			realVal := rval
			if realVal.IsNil() {
				realVal = reflect.New(rvalElemType)
			}

			err := node.scan(reflect.Indirect(realVal))
			if err != nil {
				return err
			}
			rval.Set(realVal)
			return nil

		} else {
			return node.scan(reflect.Indirect(rval))
		}

	case reflect.String:
		x, is, err := node.StringValue(nil)
		if err != nil {
			return err
		} else if !is {
			rval.Set(reflect.ValueOf(""))
			return nil
		}
		rval.Set(reflect.ValueOf(x))
		return nil

	case reflect.Bool:
		x, is, err := node.BoolValue(nil)
		if err != nil {
			return err
		} else if !is {
			rval.Set(reflect.ValueOf(true))
			return nil
		}
		rval.Set(reflect.ValueOf(x).Convert(rval.Type()))
		return nil

	case reflect.Uint64:
		x, is, err := node.UintValue(nil)
		if err != nil {
			return err
		} else if !is {
			rval.Set(reflect.ValueOf(uint64(0)))
			return nil
		}
		rval.Set(reflect.ValueOf(x).Convert(rval.Type()))
		return nil

	case reflect.Uint32:
		x, is, err := node.UintValue(nil)
		if err != nil {
			return err
		} else if !is {
			rval.Set(reflect.ValueOf(uint32(0)))
			return nil
		}
		if x > math.MaxUint32 {
			return errors.Wrap(makeScanError(node, rval.Interface()), "overflows uint32")
		}
		rval.Set(reflect.ValueOf(x).Convert(rval.Type()))
		return nil

	case reflect.Int64:
		x, is, err := node.IntValue(nil)
		if err != nil {
			return err
		} else if !is {
			rval.Set(reflect.ValueOf(int64(0)))
			return nil
		}
		rval.Set(reflect.ValueOf(x).Convert(rval.Type()))
		return nil

	case reflect.Float64:
		x, is, err := node.FloatValue(nil)
		if err != nil {
			return err
		} else if !is {
			rval.Set(reflect.ValueOf(float64(0)))
			return nil
		}
		rval.Set(reflect.ValueOf(x).Convert(rval.Type()))
		return nil

	case reflect.Map:
		keyType := rval.Type().Key()
		elemType := rval.Type().Elem()
		rval.Set(reflect.MakeMap(reflect.MapOf(keyType, elemType)).Convert(rval.Type()))

		iter := node.ChildIterator(nil, true, 10)
		defer iter.Close()

		for iter.Rewind(); iter.Valid(); iter.Next() {
			node := iter.Node()
			val := reflect.New(elemType)
			err := node.(*DBNode).scan(val)
			if err != nil {
				return err
			}
			_, key := node.Keypath().Pop()
			rKey, err := convertKeypathToType(key, keyType)
			if err != nil {
				return err
			}
			rval.SetMapIndex(rKey, val.Elem())
		}
		return nil

	case reflect.Slice:
		// Special-case []byte values
		if rval.Type().Elem().Kind() == reflect.Uint8 {
			bytes, exists, err := node.BytesValue(nil)
			if err != nil {
				return err
			} else if !exists {
				panic("this should be impossible")
			}
			rval.Set(reflect.MakeSlice(bytesType, len(bytes), len(bytes)).Convert(rval.Type()))
			reflect.Copy(rval, reflect.ValueOf(bytes))
			return nil
		}

		length, err := node.Length()
		if err != nil {
			return err
		}
		rval.Set(reflect.MakeSlice(rval.Type(), int(length), int(length)))

		iter := node.ChildIterator(nil, true, 10)
		defer iter.Close()

		i := 0
		for iter.Rewind(); iter.Valid(); iter.Next() {
			node := iter.Node()
			err := node.(*DBNode).scan(rval.Index(i))
			if err != nil {
				return err
			}
			i++
		}
		return nil

	case reflect.Array:
		elemType := rval.Type().Elem()

		// Special-case [XX]byte values
		if elemType.Kind() == reflect.Uint8 {
			bytes, exists, err := node.BytesValue(nil)
			if err != nil {
				return err
			} else if !exists {
				panic("this should be impossible: " + node.Keypath().String())
			}
			reflect.Copy(rval.Slice(0, rval.Len()), reflect.ValueOf(bytes))
			return nil
		}

		iter := node.ChildIterator(nil, true, 10)
		defer iter.Close()

		i := 0
		for iter.Rewind(); iter.Valid(); iter.Next() {
			node := iter.Node()
			err := node.(*DBNode).scan(rval.Index(i))
			if err != nil {
				return err
			}
			i++
		}
		return nil

	case reflect.Struct:
		rval.Set(reflect.Indirect(reflect.New(rval.Type())))

		z := structomancer.NewWithType(rval.Type(), StructTag)
		for _, fieldName := range z.FieldNames() {
			if fieldName == "-" {
				continue
			}
			realName := z.Field(fieldName).Name()
			ptr := rval.FieldByName(realName).Addr()

			err := node.NodeAt(Keypath(fieldName), nil).(*DBNode).scan(ptr)
			if err != nil {
				return err
			}
		}
		return nil

	case reflect.Interface:
		nodeType, valueType, _, err := node.NodeInfo(nil)
		if err != nil {
			return err
		}
		switch nodeType {
		case NodeTypeMap:
			x, is, err := node.MapValue(nil)
			if err != nil {
				return err
			} else if !is {
				return nil
			}
			rval.Set(reflect.ValueOf(x))
			return nil

		case NodeTypeSlice:
			x, is, err := node.MapValue(nil)
			if err != nil {
				return err
			} else if !is {
				return nil
			}
			rval.Set(reflect.ValueOf(x))
			return nil

		case NodeTypeValue:
			switch valueType {
			case ValueTypeUint:
				x, is, err := node.UintValue(nil)
				if err != nil {
					return err
				} else if !is {
					return nil
				}
				rval.Set(reflect.ValueOf(x))
				return nil

			case ValueTypeInt:
				x, is, err := node.IntValue(nil)
				if err != nil {
					return err
				} else if !is {
					return nil
				}
				rval.Set(reflect.ValueOf(x))
				return nil

			case ValueTypeFloat:
				x, is, err := node.FloatValue(nil)
				if err != nil {
					return err
				} else if !is {
					return nil
				}
				rval.Set(reflect.ValueOf(x))
				return nil

			case ValueTypeString:
				x, is, err := node.StringValue(nil)
				if err != nil {
					return err
				} else if !is {
					return nil
				}
				rval.Set(reflect.ValueOf(x))
				return nil

			case ValueTypeBool:
				x, is, err := node.BoolValue(nil)
				if err != nil {
					return err
				} else if !is {
					return nil
				}
				rval.Set(reflect.ValueOf(x))
				return nil
			}
		}

	default:
	}
	return makeScanError(node, rval.Interface())
}

func (tx *DBNode) Length() (uint64, error) {
	item, err := tx.tx.Get(tx.addKeyPrefix(tx.rootKeypath))
	if err == badger.ErrKeyNotFound {
		return 0, nil
	} else if err != nil {
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

	if tx.rng != nil {
		if !tx.rng.ValidForLength(length) {
			return 0, ErrInvalidRange
		}
		start, end := tx.rng.IndicesForLength(length)
		length = end - start
	}

	switch nodeType {
	case NodeTypeMap:
		return length, nil
	case NodeTypeSlice:
		return length, nil
	case NodeTypeValue:
		switch valueType {
		case ValueTypeString:
			return length, nil
		case ValueTypeBytes:
			return length, nil
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
			return errors.WithStack(ErrRangeOverNonSlice)
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
		case []byte:
			return tx.setRangeBytes(absKeypath, rng, encodedVal, spliceVal)
		case string:
			return tx.setRangeBytes(absKeypath, rng, encodedVal, []byte(spliceVal))
		case []interface{}:
			return tx.setRangeSlice(absKeypath, rng, encodedVal, spliceVal)
		default:
			return errors.New("wrong type for splice")
		}
	} else {
		return tx.setNoRange(absKeypath, val)
	}
}

func (tx *DBNode) setRangeBytes(absKeypath Keypath, rng *Range, encodedVal []byte, spliceVal []byte) error {
	if len(encodedVal) == 0 {
		encodedVal = []byte("vs")
	}
	nodeType, valueType, length, data, err := decodeNode(encodedVal)
	if err != nil {
		return err
	} else if valueType != ValueTypeString && valueType != ValueTypeBytes {
		return errors.Wrapf(ErrRangeOverNonSlice, "(keypath: %v, %v, %v)", absKeypath, nodeType, valueType)
	} else if !rng.ValidForLength(length) {
		return errors.WithStack(ErrInvalidRange)
	}

	startIdx, endIdx := rng.IndicesForLength(length)
	oldVal := data
	newLen := 2 + length - rng.Length() + uint64(len(spliceVal))
	newVal := make([]byte, newLen)
	newVal[0] = 'v'
	if valueType == ValueTypeString {
		newVal[1] = 's'
	} else {
		newVal[1] = 'z'
	}
	copy(newVal[2:], oldVal[:startIdx])
	copy(newVal[2+startIdx:], spliceVal)
	copy(newVal[2+startIdx+uint64(len(spliceVal)):], oldVal[endIdx:])
	return tx.tx.Set(tx.addKeyPrefix(absKeypath), newVal)
}

func (tx *DBNode) setRangeSlice(absKeypath Keypath, rng *Range, encodedVal []byte, spliceVal []interface{}) (err error) {
	defer errors.Annotate(&err, "setRangeSlice")

	nodeType, valueType, oldLen, _, err := decodeNode(encodedVal)
	if err != nil {
		return err
	} else if nodeType != NodeTypeSlice {
		return errors.Wrapf(ErrRangeOverNonSlice, "(keypath: %v, %v, %v)", absKeypath, nodeType, valueType)
	} else if !rng.ValidForLength(oldLen) {
		return ErrInvalidRange
	}

	absKeypath = tx.addKeyPrefix(absKeypath)

	newLen := oldLen - rng.Length() + uint64(len(spliceVal))
	shrink := newLen < oldLen
	startIdx, endIdx := rng.IndicesForLength(oldLen)

	// Delete deleted items
	if startIdx != endIdx {
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
			if startIdx > 0 {
				endKeypath = absKeypath.PushIndex(startIdx - 1).Push([]byte{0xff})
			}
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
	err = walkGoValue(spliceVal, func(nodeKeypath Keypath, val interface{}) (keepRecursing bool, _ error) {
		nodeKeypath = nodeKeypath.Copy()
		if len(nodeKeypath) == 0 {
			encoded, err := encodeNode(NodeTypeSlice, ValueTypeInvalid, newLen, nil)
			if err != nil {
				return false, err
			}

			err = tx.tx.Set(absKeypath, encoded)
			if err != nil {
				return false, err
			}
			return true, nil
		}

		absNodeKeypath := absKeypath
		oldIdx := DecodeSliceIndex(nodeKeypath[:8])
		newIdx := oldIdx + startIdx
		copy(nodeKeypath[:8], EncodeSliceIndex(newIdx))
		absNodeKeypath = absKeypath.Push(nodeKeypath)

		encoded, err := encodeGoValue(val)
		if err != nil {
			return false, err
		}

		tx.diff.Add(tx.rmKeyPrefix(absNodeKeypath))

		err = tx.tx.Set(absNodeKeypath, encoded)
		if err != nil {
			return false, errors.Wrapf(err, "can't set keypath %v", absNodeKeypath)
		}
		return true, nil
	})
	return err
}

func (tx *DBNode) setNoRange(absKeypath Keypath, value interface{}) error {
	err := tx.delete(absKeypath, nil)
	if err != nil {
		return err
	}

	numParts := absKeypath.NumParts()

	// When setting a value in a map, we need to check to see if the key is new,
	// and if so, increment the map's length by 1
	if numParts > 0 {
		absParentKeypath, _ := absKeypath.Pop()

		exists, err := tx.exists(absParentKeypath)
		if err != nil {
			return err
		}

		if exists {
			nodeType, _, parentLength, err := tx.nodeInfo(absParentKeypath)
			if err != nil {
				return err
			}
			if nodeType == NodeTypeMap {
				exists, err := tx.exists(absKeypath)
				if err != nil {
					return err
				}
				if !exists {
					encoded, err := encodeNode(NodeTypeMap, ValueTypeInvalid, parentLength+1, nil)
					if err != nil {
						return err
					}

					err = tx.tx.Set(tx.addKeyPrefix(absParentKeypath), encoded)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	// Set value types for intermediate keypaths in case they don't exist
	for i := 0; i < numParts; i++ {
		partialKeypath := tx.addKeyPrefix(absKeypath.FirstNParts(i))

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
		}
		//  else {
		// 	nodeType, _, _, err := tx.NodeInfo(partialKeypath)
		// 	if err != nil {
		// 		return err
		// 	}
		// 	if nodeType == NodeTypeValue {
		// 		encoded, err := encodeNode(NodeTypeMap, ValueTypeInvalid, 1, nil)
		// 		if err != nil {
		// 			return err
		// 		}
		// 		err = tx.tx.Set(partialKeypath, encoded)
		// 		if err != nil {
		// 			return err
		// 		}
		// 	}
		// }
		// if len(partialKeypath) == len(tx.keyPrefix)+len(tx.rootKeypath) {
		// 	break
		// }
		// partialKeypath, _ = partialKeypath.Pop()
		// if partialKeypath == nil {
		// 	partialKeypath = tx.addKeyPrefix(partialKeypath)
		// }
	}

	err = walkGoValue(value, func(nodeKeypath Keypath, nodeValue interface{}) (keepRecursing bool, _ error) {
		absNodeKeypath := absKeypath
		if len(nodeKeypath) != 0 {
			absNodeKeypath = absKeypath.Push(nodeKeypath)
		}

		if asNode, isNode := nodeValue.(Node); isNode {
			return true, tx.setNode(absNodeKeypath, asNode)

		} else if asMarshaler, isMarshaler := nodeValue.(StateBytesMarshaler); isMarshaler {
			encoded, err := asMarshaler.MarshalStateBytes()
			if err != nil {
				return false, err
			}
			encoded, err = encodeGoValue(encoded)
			if err != nil {
				return false, err
			}
			return false, tx.tx.Set(tx.addKeyPrefix(absNodeKeypath), encoded)
		}

		// @@TODO: diff logic sometimes overstates the change or duplicates keypaths
		tx.diff.Add(absNodeKeypath)

		encoded, err := encodeGoValue(nodeValue)
		if err != nil {
			return false, err
		}
		return true, tx.tx.Set(tx.addKeyPrefix(absNodeKeypath), encoded)
	})
	return err
}

func (tx *DBNode) encodedBytes(absKeypath Keypath) ([]byte, error) {
	item, err := tx.tx.Get(tx.addKeyPrefix(absKeypath))
	if err != nil {
		return nil, err
	}
	return item.ValueCopy(nil)
}

func (n *DBNode) innerNode(relKeypath Keypath) Node {
	return nil
}

func (tx *DBNode) setNode(absKeypath Keypath, node Node) error {
	iter := node.Iterator(nil, true, 10)
	defer iter.Close()

	// baseKeypath := tx.rootKeypath.Push(relKeypath)

	for iter.Rewind(); iter.Valid(); iter.Next() {
		child := iter.Node()
		childRelKeypath := child.Keypath().RelativeTo(node.Keypath())

		// Other Node trees set inside of the parent node
		if innerNode := node.innerNode(childRelKeypath); innerNode != nil {
			err := tx.setNode(absKeypath.Push(childRelKeypath), innerNode)
			if err != nil {
				return err
			}
			continue
		}

		// DBNodes
		if asDBNode, isDBNode := child.(interface {
			encodedBytes(keypath Keypath) ([]byte, error)
		}); isDBNode {
			encoded, err := asDBNode.encodedBytes(child.Keypath())
			if err != nil {
				return err
			}
			err = tx.tx.Set(tx.addKeyPrefix(absKeypath.Push(childRelKeypath)), encoded)
			if err != nil {
				return err
			}
			continue
		}

		// Everything else
		nodeType, valueType, length, err := node.NodeInfo(childRelKeypath)
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
		encoded, err := encodeNode(nodeType, valueType, length, val)
		if err != nil {
			return err
		}
		err = tx.tx.Set(tx.addKeyPrefix(absKeypath.Push(childRelKeypath)), encoded)
		if err != nil {
			return err
		}

	}
	return nil
}

func (tx *DBNode) Delete(relKeypath Keypath, rng *Range) (err error) {
	return tx.delete(tx.rootKeypath.Push(relKeypath), rng)
}

func (tx *DBNode) delete(absKeypath Keypath, rng *Range) (err error) {
	defer errors.Annotate(&err, "DBNode#Delete(%v, %v)", absKeypath, rng)

	if rng == nil {
		rng = tx.rng
	} else if rng != nil && tx.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return ErrInvalidRange
	}

	item, err := tx.tx.Get(tx.addKeyPrefix(absKeypath))
	if err != nil && err == badger.ErrKeyNotFound {
		return nil
	} else if err != nil {
		return err
	}

	val, err := item.ValueCopy(nil)
	if err != nil {
		return err
	}

	rootNodeType, valueType, length, data, err := decodeNode(val)
	if err != nil {
		return err
	}

	// If it's a simple NodeTypeValue, handle it without iteration
	if rootNodeType == NodeTypeValue && rng != nil {
		return tx.deleteRangeOfValue(absKeypath, rng, rootNodeType, valueType, length, data)
	}

	// Convert non-range deletes into range deletes to reduce code
	if rng == nil {
		absParentKeypath, keypathToDelete := absKeypath.Pop()
		parentNodeType, _, _, err := tx.nodeInfo(absParentKeypath)
		if err != nil {
			return err
		}

		if parentNodeType == NodeTypeMap {
			idx, err := tx.indexOfMapSubkey(absParentKeypath, keypathToDelete)
			if errors.Cause(err) == ErrNilKeypath {
				rng = &Range{0, int64(length)}
			} else if err != nil {
				return err
			}
			rng = &Range{int64(idx), int64(idx) + 1}

		} else if parentNodeType == NodeTypeSlice {
			idx := int64(DecodeSliceIndex(keypathToDelete))
			rng = &Range{idx, idx + 1}
		}
		return tx.delete(absParentKeypath, rng)
	}

	// Delete child nodes
	{
		tx.scanChildrenForward(rootNodeType, absKeypath, rng, length, false, func(absChildKeypath Keypath, item *badger.Item) error {
			// This .Copy() is necessary.  See https://github.com/dgraph-io/badger/issues/494
			err := tx.tx.Delete(tx.addKeyPrefix(absChildKeypath.Copy()))
			if err != nil {
				return errors.Wrapf(err, "can't delete keypath %v", absChildKeypath)
			}
			tx.diff.Remove(absChildKeypath)
			return nil
		})
	}

	err = tx.tx.Delete(tx.addKeyPrefix(absKeypath))
	if err != nil {
		return err
	}
	tx.diff.Remove(absKeypath)

	_, endIdx := rng.IndicesForLength(length)

	// Re-number the trailing entries if the root node is a slice
	if rootNodeType == NodeTypeSlice && endIdx < length {
		renumberRange := &Range{int64(endIdx), int64(length)}
		delta := -int64(rng.Length())

		err := tx.scanChildrenForward(NodeTypeSlice, absKeypath, renumberRange, length, true, func(absChildKeypath Keypath, item *badger.Item) error {
			valueBuf, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			newAbsChildKeypath := renumberSliceIndexKeypath(absKeypath, absChildKeypath, delta)

			err = tx.tx.Set(tx.addKeyPrefix(newAbsChildKeypath), valueBuf)
			if err != nil {
				return errors.Wrapf(err, "can't set keypath %v", newAbsChildKeypath)
			}
			// This .Copy() is necessary.  See https://github.com/dgraph-io/badger/issues/494
			err = tx.tx.Delete(tx.addKeyPrefix(absChildKeypath.Copy()))
			if err != nil {
				return errors.Wrapf(err, "can't delete keypath %v", absChildKeypath)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Set new length
	if rng != nil && (rootNodeType == NodeTypeSlice || rootNodeType == NodeTypeMap) {
		newLen := length - rng.Length()
		encoded, err := encodeNode(rootNodeType, ValueTypeInvalid, newLen, nil)
		if err != nil {
			return err
		}
		err = tx.tx.Set(tx.addKeyPrefix(absKeypath), encoded)
		if err != nil {
			return err
		}
	}

	// @@TODO: make sure that everywhere we're putting things into the Diff, we use tx.rmKeyPrefix(...)

	return nil
}

func (tx *DBNode) deleteRangeOfValue(absKeypath Keypath, rng *Range, rootNodeType NodeType, valueType ValueType, length uint64, data []byte) error {
	if valueType != ValueTypeString && valueType != ValueTypeBytes {
		return errors.Wrapf(ErrRangeOverNonSlice, "(keypath: %v, %v, %v)", tx.addKeyPrefix(absKeypath), rootNodeType, valueType)
	} else if !rng.ValidForLength(length) {
		return ErrInvalidRange
	}

	v, err := decodeGoValue(rootNodeType, valueType, length, nil, data)
	if err != nil {
		return err
	}

	startIdx, endIdx := rng.IndicesForLength(length)

	if valueType == ValueTypeString {
		s := v.(string)
		s = s[:startIdx] + s[endIdx:]

		// @@TODO: add a "modified" field to the diff?
		// tx.diff.Remove(tx.rmKeyPrefix(absKeypath))
		return tx.Set(absKeypath, nil, s)
	} else {
		s := v.([]byte)
		s = append(s[:startIdx], s[endIdx:]...)

		// @@TODO: add a "modified" field to the diff?
		// tx.diff.Remove(tx.rmKeyPrefix(absKeypath))
		return tx.Set(absKeypath, nil, s)
	}
}

func (tx *DBNode) Diff() *Diff {
	return tx.diff
}

func (tx *DBNode) ResetDiff() {
	tx.diff = NewDiff()
}

func (tx *DBNode) CopyToMemory(relKeypath Keypath, rng *Range) (n Node, err error) {
	absKeypath := tx.rootKeypath.Push(relKeypath)
	return tx.copyToMemory(absKeypath, rng)
}

func (tx *DBNode) copyToMemory(absKeypath Keypath, rng *Range) (n Node, err error) {
	defer errors.Annotate(&err, "CopyToMemory (absKeypath: %v, range: %v)", absKeypath, rng)

	if rng == nil {
		rng = tx.rng
	} else if rng != nil && tx.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return nil, ErrInvalidRange
	}

	item, err := tx.tx.Get(tx.addKeyPrefix(absKeypath))
	if errors.Cause(err) == badger.ErrKeyNotFound {
		return nil, errors.Err404
	} else if err != nil {
		return nil, err
	}

	valBytes, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}

	rootNodeType, valueType, length, data, err := decodeNode(valBytes)
	if err != nil {
		return nil, errors.Wrapf(err, "(data: %v)", string(valBytes))
	}

	if rng != nil && !rng.ValidForLength(length) {
		return nil, ErrInvalidRange
	}

	mNode := NewMemoryNode()

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

	// if len(tx.rmKeyPrefix(rootKeypath)) > 0 {
	mNode.keypaths = append(mNode.keypaths, Keypath(nil))
	// }
	mNode.nodeTypes[""] = rootNodeType

	var startIdx uint64

	if rootNodeType == NodeTypeMap {
		if rng != nil {
			mNode.contentLengths[""] = rng.Length()
		} else {
			mNode.contentLengths[""] = length
		}
	} else if rootNodeType == NodeTypeSlice {
		if rng != nil {
			mNode.contentLengths[""] = rng.Length()
			startIdx, _ = rng.IndicesForLength(length)
		} else {
			mNode.contentLengths[""] = length
		}
	}

	var newKeypaths []Keypath
	var valBuf []byte

	err = tx.scanChildrenForward(rootNodeType, absKeypath, rng, length, true, func(absChildKeypath Keypath, item *badger.Item) error {
		relKeypath := absChildKeypath.RelativeTo(absKeypath).Copy()

		// If we're ranging over a slice, transpose its indices so that they start from 0
		if rootNodeType == NodeTypeSlice && rng != nil {
			relKeypath = renumberSliceIndexKeypath(absKeypath, absChildKeypath, -int64(startIdx))
		}

		err := item.Value(func(bs []byte) error {
			valBuf = bs
			return nil
		})
		if err != nil {
			return err
		}

		nodeType, valueType, length, data, err := decodeNode(valBuf)
		if err != nil {
			return err
		}

		decoded, err := decodeGoValue(nodeType, valueType, length, nil, data)
		if err != nil {
			return err
		}

		newKeypaths = append(newKeypaths, relKeypath)
		mNode.nodeTypes[string(relKeypath)] = nodeType
		if nodeType == NodeTypeSlice || nodeType == NodeTypeMap {
			mNode.contentLengths[string(relKeypath)] = length
		} else if nodeType == NodeTypeValue {
			mNode.values[string(relKeypath)] = decoded
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	mNode.addKeypaths(newKeypaths)
	mNode.diff = tx.diff.Copy()
	return mNode, nil
}

func (tx *DBNode) DebugPrint(printFn func(inFormat string, args ...interface{}), newlines bool, indentLevel int) {
	iter := tx.Iterator(nil, true, 10)
	defer iter.Close()

	if newlines {
		oldPrintFn := printFn
		printFn = func(inFormat string, args ...interface{}) { oldPrintFn(inFormat+"\n", args...) }
	}

	indent := strings.Repeat(" ", 4*indentLevel)

	file, line := getFileAndLine()
	printFn(indent+"DBNode (root keypath: %v) (%v:%v) {", tx.rootKeypath, file, line)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		node := iter.Node()
		valBytes, err := tx.encodedBytes(node.Keypath())
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

		if nodeType == NodeTypeValue {
			switch valueType {
			case ValueTypeBytes:
				printFn(indent+"    %s: (%v, length %v) %v", node.Keypath(), valueType, length, hex.EncodeToString(val.([]byte)))
			case ValueTypeString:
				printFn(indent+"    %s: (%v, length %v) %v", node.Keypath(), valueType, length, val)
			case ValueTypeInt:
				printFn(indent+"    %s: (%v) %v", node.Keypath(), valueType, val)
			case ValueTypeUint:
				printFn(indent+"    %s: (%v) %v", node.Keypath(), valueType, val)
			case ValueTypeBool:
				printFn(indent+"    %s: (%v) %v", node.Keypath(), valueType, val)
			case ValueTypeFloat:
				printFn(indent+"    %s: (%v) %v", node.Keypath(), valueType, val)
			case ValueTypeNil:
				printFn(indent+"    %s: (%v)", node.Keypath(), valueType)
			case ValueTypeInvalid:
				printFn(indent+"    %s: (%v)", node.Keypath(), valueType)
			}
		} else {
			printFn(indent+"    %s: (%v, length %v)", node.Keypath(), nodeType, length)
		}

	}
	printFn(indent + "}")
}

func (t *VersionedDBTree) CopyVersion(dstVersion, srcVersion Version) error {
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
			startKeypath = keypathPrefix.PushIndex(uint64(rng.Start))
			endKeypath = keypathPrefix.PushIndex(uint64(rng.End))
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

func (n *DBNode) Iterator(relKeypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	absKeypath := n.Keypath().Push(relKeypath)
	return n.iterator(absKeypath, prefetchValues, prefetchSize)
}

func (n *DBNode) iterator(absKeypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	// Badger doesn't allow more than one iterator open inside of a RW transaction
	if n.activeIterator.mutable && n.activeIterator.iter != nil {
		return newReusableIterator(n.activeIterator.iter, absKeypath, n)
	}

	opts := badger.DefaultIteratorOptions
	opts.Reverse = false
	opts.PrefetchValues = prefetchValues
	opts.PrefetchSize = prefetchSize
	badgerIter := n.tx.NewIterator(opts)
	iter := newIteratorFromBadgerIterator(badgerIter, absKeypath, n)
	n.activeIterator.iter = iter
	return iter
}

func (n *DBNode) ChildIterator(relKeypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	absKeypath := n.Keypath().Push(relKeypath)
	return n.childIterator(absKeypath, prefetchValues, prefetchSize)
}

func (n *DBNode) childIterator(absKeypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	dbIter := n.iterator(absKeypath, prefetchValues, prefetchSize)
	return &dbChildIterator{
		Iterator:           dbIter,
		numAbsKeypathParts: dbIter.RootKeypath().NumParts(),
	}
}

func (n *DBNode) DepthFirstIterator(relKeypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	absKeypath := n.Keypath().Push(relKeypath)
	return n.depthFirstIterator(absKeypath, prefetchValues, prefetchSize)
}

func (n *DBNode) depthFirstIterator(absKeypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	opts := badger.DefaultIteratorOptions
	opts.Reverse = true
	opts.PrefetchValues = prefetchValues
	opts.PrefetchSize = prefetchSize
	iter := n.tx.NewIterator(opts)

	scanPrefix := n.addKeyPrefix(absKeypath)
	if len(scanPrefix) != len(n.keyPrefix) {
		scanPrefix = append(scanPrefix, KeypathSeparator[0])
	}

	return &dbDepthFirstIterator{
		iter:           iter,
		absRootKeypath: absKeypath,
		scanPrefix:     scanPrefix,
		tx:             n.tx,
		rootNode:       n,
		iterNode:       &DBNode{tx: n.tx},
	}
}

type Indexer interface {
	IndexNode(relKeypath Keypath, state Node) (Keypath, Node, error)
}

func prettyJSON(x interface{}) string {
	j, _ := json.MarshalIndent(x, "", "    ")
	return string(j)
}

func (t *VersionedDBTree) BuildIndex(version *Version, node Node, indexName Keypath, indexer Indexer) (err error) {
	defer errors.Annotate(&err, "BuildIndex")

	// @@TODO: ensure NodeType is map or slice
	// @@TODO: don't use a map[][] to count children, put it in Badger
	index := t.IndexAtVersion(version, node.Keypath(), indexName, true)
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

		indexKey, indexNode, err := indexer.IndexNode(relKeypath, childNode)
		if err != nil {
			return err
		} else if indexKey == nil || indexNode == nil {
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

		err = index.Set(relKeypath, nil, indexNode)
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

func (n *DBNode) scanChildrenForward(
	rootNodeType NodeType,
	absParentKeypath Keypath,
	rng *Range,
	length uint64,
	prefetchValues bool,
	fn func(absKeypath Keypath, item *badger.Item) error,
) error {
	var absStartKeypath Keypath
	var absEndKeypath Keypath
	var endIdx uint64

	iter := n.iterator(absParentKeypath, prefetchValues, 10)
	defer iter.Close()

	if rootNodeType == NodeTypeMap {
		if rng != nil {
			if !rng.ValidForLength(length) {
				return ErrInvalidRange
			} else if rng.Length() == 0 {
				return nil
			}
			var startIdx uint64
			startIdx, endIdx = rng.IndicesForLength(length)
			absStartKeypath = n.nthMapSubkey(absParentKeypath, startIdx)
			absEndKeypath = n.nthMapSubkey(absParentKeypath, endIdx)
			iter.seekTo(absStartKeypath)

		} else {
			iter.Rewind()
			iter.Next()
		}

	} else if rootNodeType == NodeTypeSlice {
		if rng != nil {
			if !rng.ValidForLength(length) {
				return ErrInvalidRange
			} else if rng.Length() == 0 {
				return nil
			}
			startIdx, endIdx := rng.IndicesForLength(length)
			absStartKeypath = absParentKeypath.PushIndex(startIdx)
			absEndKeypath = absParentKeypath.PushIndex(endIdx)
			iter.seekTo(absStartKeypath)

		} else {
			iter.Rewind()
			iter.Next()
		}
	} else {
		return errors.New("scanChildrenForward can only be called on a map or slice")
	}

	for ; iter.Valid(); iter.Next() {
		var item *badger.Item
		switch i := iter.(type) {
		case *dbIterator:
			item = i.iter.Item()
		case *reusableIterator:
			item = i.badgerIter.Item()
		default:
			panic("this should never happen")
		}
		node := iter.Node()

		// If we have a range, we have to figure out when to stop iterating
		if rng != nil {
			if node.Keypath().Equals(absEndKeypath) {
				break
			}
		}

		err := fn(node.Keypath(), item)
		if err != nil {
			return err
		}
	}
	return nil
}

// NthSubkey finds the Nth direct subkey in a map.
func (node *DBNode) NthMapSubkey(relKeypathOfMap Keypath, n uint64) Keypath {
	absKeypathOfMap := node.Keypath().Push(relKeypathOfMap)
	return node.nthMapSubkey(absKeypathOfMap, n).RelativeTo(node.Keypath())
}

func (node *DBNode) nthMapSubkey(absKeypathOfMap Keypath, n uint64) Keypath {
	iter := node.childIterator(absKeypathOfMap, false, 0)
	defer iter.Close()

	var idx uint64
	for iter.Rewind(); iter.Valid(); iter.Next() {
		child := iter.Node()
		if child == nil {
			return nil
		}

		if idx == n {
			return child.Keypath()
		}
		idx++
	}
	return nil
}

func (node *DBNode) IndexOfMapSubkey(relKeypathOfMap Keypath, subkey Keypath) (uint64, error) {
	absKeypathOfMap := node.Keypath().Push(relKeypathOfMap)
	return node.indexOfMapSubkey(absKeypathOfMap, subkey)
}

func (node *DBNode) indexOfMapSubkey(absKeypathOfMap Keypath, subkey Keypath) (uint64, error) {
	nodeType, _, _, err := node.nodeInfo(absKeypathOfMap)
	if err != nil {
		return 0, err
	} else if nodeType != NodeTypeMap {
		return 0, ErrWrongType
	} else if len(subkey) == 0 {
		return 0, ErrNilKeypath
	}

	iter := node.childIterator(absKeypathOfMap, false, 0)
	defer iter.Close()

	var idx uint64
	for iter.Rewind(); iter.Valid(); iter.Next() {
		node := iter.Node()
		if node == nil {
			break
		}

		_, relKeypath := node.Keypath().Pop()
		if relKeypath.Equals(subkey) {
			return idx, nil
		}
		idx++
	}
	return 0, errors.Err404
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
			v, ok := value.(uint64)
			if !ok {
				v = toUint64(value)
			}
			encoded := make([]byte, 10)
			encoded[0] = 'v'
			encoded[1] = 'u'
			binary.LittleEndian.PutUint64(encoded[2:10], v)
			return encoded, nil

		case ValueTypeInt:
			v, ok := value.(int64)
			if !ok {
				v = toInt64(value)
			}
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
			v, ok := value.(float64)
			if !ok {
				v = toFloat64(value)
			}
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

		case ValueTypeBytes:
			v := value.([]byte)
			encoded := make([]byte, 2+len(v))
			encoded[0] = 'v'
			encoded[1] = 'z'
			copy(encoded[2:], v)
			return encoded, nil

		case ValueTypeNil:
			encoded := []byte("v0")
			return encoded, nil

		default:
			return nil, errors.Wrapf(ErrNodeEncoding, "(%v, %v, %v, %v)", nodeType, valueType, length, value)
		}
	default:
		return nil, errors.Wrapf(ErrNodeEncoding, "(%v, %v, %v, %v)", nodeType, valueType, length, value)
	}
}

func toInt64(val interface{}) int64 {
	switch v := val.(type) {
	case int32:
		return int64(v)
	case int16:
		return int64(v)
	case int8:
		return int64(v)
	case int:
		return int64(v)
	default:
		panic("not an int")
	}
}

func toUint64(val interface{}) uint64 {
	switch v := val.(type) {
	case uint32:
		return uint64(v)
	case uint16:
		return uint64(v)
	case uint8:
		return uint64(v)
	case uint:
		return uint64(v)
	default:
		panic("not a uint")
	}
}

func toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float32:
		return float64(v)
	default:
		panic("not a float")
	}
}

func renumberSliceIndexKeypath(rootKeypath, absKeypath Keypath, delta int64) (newAbsKeypath Keypath) {
	relKeypath := absKeypath.RelativeTo(rootKeypath).Copy()
	oldIdx := DecodeSliceIndex(relKeypath[:8])
	newIdx := uint64(int64(oldIdx) + delta)
	copy(relKeypath[:8], EncodeSliceIndex(newIdx))
	return rootKeypath.Push(relKeypath)
}

func encodeGoValue(nodeValue interface{}) ([]byte, error) {
	switch nv := nodeValue.(type) {
	case map[string]interface{}:
		return encodeNode(NodeTypeMap, 0, uint64(len(nv)), nil)
	case []interface{}:
		return encodeNode(NodeTypeSlice, 0, uint64(len(nv)), nil)
	case bool:
		return encodeNode(NodeTypeValue, ValueTypeBool, 0, nv)
	case uint64, uint32, uint16, uint8, uint:
		return encodeNode(NodeTypeValue, ValueTypeUint, 0, nv)
	case int64, int32, int16, int8, int:
		return encodeNode(NodeTypeValue, ValueTypeInt, 0, nv)
	case float64:
		return encodeNode(NodeTypeValue, ValueTypeFloat, 0, nv)
	case string:
		return encodeNode(NodeTypeValue, ValueTypeString, 0, nv)
	case []byte:
		return encodeNode(NodeTypeValue, ValueTypeBytes, 0, nv)
	case nil:
		return encodeNode(NodeTypeValue, ValueTypeNil, 0, nv)
	default:
		rval := reflect.ValueOf(nodeValue)
		switch {
		case rval.Kind() == reflect.Struct:
			z := structomancer.NewWithType(rval.Type(), StructTag)
			return encodeNode(NodeTypeMap, 0, uint64(z.NumFields()), nil)

		case rval.Kind() == reflect.Ptr && rval.Type().Elem().Kind() == reflect.Struct:
			z := structomancer.NewWithType(rval.Type(), StructTag)
			return encodeNode(NodeTypeMap, 0, uint64(z.NumFields()), nil)

		case rval.Kind() == reflect.Map:
			return encodeNode(NodeTypeMap, 0, uint64(rval.Len()), nil)

		case rval.Kind() == reflect.Slice:
			if rval.Type().Elem().Kind() == reflect.Uint8 {
				bs := make([]byte, rval.Len())
				reflect.Copy(reflect.ValueOf(bs), rval)
				return encodeNode(NodeTypeValue, ValueTypeBytes, uint64(rval.Len()), bs)
			}
			return encodeNode(NodeTypeSlice, 0, uint64(rval.Len()), nil)

		case rval.Kind() == reflect.Array:
			if rval.Type().Elem().Kind() == reflect.Uint8 {
				bs := make([]byte, rval.Len())
				reflect.Copy(reflect.ValueOf(bs), rval) //.Slice(0, rval.Len()))
				return encodeNode(NodeTypeValue, ValueTypeBytes, uint64(rval.Len()), bs)
			}
			return encodeNode(NodeTypeSlice, 0, uint64(rval.Len()), nil)
		}
		return nil, errors.Errorf("cannot encode Go value of type (%T)", nodeValue)
	}
}

func decodeNode(val []byte) (NodeType, ValueType, uint64, []byte, error) {
	if len(val) == 0 {
		return NodeTypeInvalid, ValueTypeInvalid, 0, nil, errors.Wrapf(ErrNodeEncoding, "len(val) == 0")
	}
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
		case 'z':
			return NodeTypeValue, ValueTypeBytes, uint64(len(val[2:])), val[2:], nil
		case '0':
			return NodeTypeValue, ValueTypeNil, 0, nil, nil
		default:
			return NodeTypeInvalid, ValueTypeInvalid, 0, nil, errors.Wrapf(ErrNodeEncoding, "(val: %v / %v)", val, string(val))
		}
	default:
		return NodeTypeInvalid, ValueTypeInvalid, 0, nil, errors.Wrapf(ErrNodeEncoding, "(val: %v / %v)", val, string(val))
	}
}

func decodeGoValue(nodeType NodeType, valueType ValueType, length uint64, rng *Range, data []byte) (interface{}, error) {
	switch nodeType {
	case NodeTypeMap:
		if rng != nil {
			if !rng.ValidForLength(length) {
				return nil, errors.WithStack(ErrInvalidRange)
			}
			length = rng.Length()
		}
		return make(map[string]interface{}), nil

	case NodeTypeSlice:
		if rng != nil {
			if !rng.ValidForLength(length) {
				return nil, errors.WithStack(ErrInvalidRange)
			}
			length = rng.Length()
		}
		return make([]interface{}, length), nil

	case NodeTypeValue:
		switch valueType {
		case ValueTypeBool:
			if rng != nil {
				return nil, errors.Wrapf(ErrRangeOverNonSlice, "(%v, %v)", nodeType, valueType)
			}
			if data[0] == byte(1) {
				return true, nil
			} else {
				return false, nil
			}

		case ValueTypeUint:
			if rng != nil {
				return nil, errors.Wrapf(ErrRangeOverNonSlice, "(%v, %v)", nodeType, valueType)
			}
			return binary.LittleEndian.Uint64(data), nil

		case ValueTypeInt:
			if rng != nil {
				return nil, errors.Wrapf(ErrRangeOverNonSlice, "(%v, %v)", nodeType, valueType)
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
				return nil, errors.Wrapf(ErrRangeOverNonSlice, "(%v, %v)", nodeType, valueType)
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

		case ValueTypeBytes:
			if rng != nil {
				if !rng.ValidForLength(length) {
					return nil, errors.WithStack(ErrInvalidRange)
				}
				startIdx, endIdx := rng.IndicesForLength(length)
				return data[startIdx:endIdx], nil
			}
			return data, nil

		case ValueTypeNil:
			return nil, nil

		default:
			return nil, errors.WithStack(ErrNodeEncoding)
		}

	default:
		return nil, errors.WithStack(ErrNodeEncoding)
	}
}
