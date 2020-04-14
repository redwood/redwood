package tree

import (
	"encoding/binary"
	"fmt"

	"github.com/pkg/errors"
)

func EncodeSliceIndex(x uint64) Keypath {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, x)
	return b
}

func DecodeSliceIndex(k Keypath) uint64 {
	return binary.BigEndian.Uint64(k)
}

func EncodeSliceLen(x uint64) Keypath {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, x)
	return b
}

func DecodeSliceLen(k Keypath) uint64 {
	return binary.LittleEndian.Uint64(k)
}

func walkGoValue(tree interface{}, fn func(keypath Keypath, val interface{}) error) error {
	type item struct {
		val     interface{}
		keypath Keypath
	}

	stack := []item{{val: tree, keypath: nil}}
	var current item

	for len(stack) > 0 {
		current = stack[0]
		stack = stack[1:]

		err := fn(current.keypath, current.val)
		if err != nil {
			return err
		}

		if asMap, isMap := current.val.(map[string]interface{}); isMap {
			for key := range asMap {
				stack = append(stack, item{
					val:     asMap[key],
					keypath: current.keypath.Push(Keypath(key)),
				})
			}

		} else if asSlice, isSlice := current.val.([]interface{}); isSlice {
			for i := range asSlice {
				stack = append(stack, item{
					val:     asSlice[i],
					keypath: current.keypath.Push(EncodeSliceIndex(uint64(i))),
				})
			}
		}
	}
	return nil
}

func setValueAtKeypath(x interface{}, keypath Keypath, val interface{}, clobber bool) interface{} {
	if len(keypath) == 0 {
		return val
	}

	var cur interface{} = x
	var key Keypath
	for {
		key, keypath = keypath.Shift()
		if keypath == nil {
			break
		}

		if asMap, isMap := cur.(map[string]interface{}); isMap {
			var exists bool
			cur, exists = asMap[string(key)]
			if exists {
				if clobber {
					asMap[string(key)] = make(map[string]interface{})
					cur = asMap[string(key)]
				}
			}

		} else if asSlice, isSlice := cur.([]interface{}); isSlice {
			cur = asSlice[DecodeSliceIndex(key)]
		} else if asNode, isNode := cur.(Node); isNode {
			asNode.Set(Keypath(key), nil, make(map[string]interface{}))
			cur = asNode.NodeAt(Keypath(key), nil)
		} else {
			panic(fmt.Sprintf("bad 2: %T %v", cur, key))
		}
	}

	if asMap, isMap := cur.(map[string]interface{}); isMap {
		asMap[string(key)] = val
	} else if asSlice, isSlice := cur.([]interface{}); isSlice {
		asSlice[DecodeSliceIndex(key)] = val
	} else if asNode, isNode := cur.(Node); isNode {
		asNode.Set(Keypath(key), nil, val)
	} else {
		panic(fmt.Sprintf("bad 3: %T %v", cur, key))
	}
	return x
}

func annotate(err *error, msg string, args ...interface{}) {
	if *err != nil {
		*err = errors.Wrapf(*err, msg, args...)
	}
}
