package state

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"runtime"
	"strconv"

	"github.com/brynbellomy/go-structomancer"
	"github.com/pkg/errors"
)

const StructTag = "tree"

func getFileAndLine() (string, int) {
	pc, _, _, _ := runtime.Caller(2)
	fn := runtime.FuncForPC(pc)
	return fn.FileLine(pc)
}

func EncodeSliceIndex(x uint64) Keypath {
	enc := []byte(strconv.FormatUint(x, 10))
	pad := bytes.Repeat([]byte("0"), 8-len(enc))
	enc = append(pad, enc...)
	return Keypath(enc)
}

func DecodeSliceIndex(k Keypath) uint64 {
	x, err := strconv.ParseUint(string(k), 10, 64)
	if err != nil {
		panic(err)
	}
	return x
}

func EncodeSliceLen(x uint64) Keypath {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, x)
	return b
}

func DecodeSliceLen(k Keypath) uint64 {
	return binary.LittleEndian.Uint64(k)
}

func convertKeypathToType(keypath Keypath, typ reflect.Type) (reflect.Value, error) {
	var val reflect.Value

	switch typ.Kind() {
	case reflect.Uint:
		i, err := strconv.ParseUint(string(keypath), 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		val = reflect.ValueOf(uint(i))

	case reflect.Uint64:
		i, err := strconv.ParseUint(string(keypath), 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		val = reflect.ValueOf(uint64(i))

	case reflect.Uint32:
		i, err := strconv.ParseUint(string(keypath), 10, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		val = reflect.ValueOf(uint32(i))

	case reflect.Uint16:
		i, err := strconv.ParseUint(string(keypath), 10, 16)
		if err != nil {
			return reflect.Value{}, err
		}
		val = reflect.ValueOf(uint16(i))

	case reflect.Uint8:
		i, err := strconv.ParseUint(string(keypath), 10, 8)
		if err != nil {
			return reflect.Value{}, err
		}
		val = reflect.ValueOf(uint8(i))

	case reflect.Int:
		i, err := strconv.ParseInt(string(keypath), 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		val = reflect.ValueOf(int(i))

	case reflect.Int64:
		i, err := strconv.ParseInt(string(keypath), 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		val = reflect.ValueOf(int64(i))

	case reflect.Int32:
		i, err := strconv.ParseInt(string(keypath), 10, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		val = reflect.ValueOf(int32(i))

	case reflect.Int16:
		i, err := strconv.ParseInt(string(keypath), 10, 16)
		if err != nil {
			return reflect.Value{}, err
		}
		val = reflect.ValueOf(int16(i))

	case reflect.Int8:
		i, err := strconv.ParseInt(string(keypath), 10, 8)
		if err != nil {
			return reflect.Value{}, err
		}
		val = reflect.ValueOf(int8(i))

	case reflect.String:
		val = reflect.ValueOf(string(keypath))

	case reflect.Slice:
		if typ.Elem().Kind() != reflect.Uint8 {
			return reflect.Value{}, errors.Errorf("could not convert tree.Keypath to %v", typ)
		}
		val = reflect.MakeSlice(typ, len(keypath), len(keypath))
		reflect.Copy(val, reflect.ValueOf([]byte(keypath)))

	case reflect.Array:
		if typ.Elem().Kind() != reflect.Uint8 {
			return reflect.Value{}, errors.Errorf("could not convert tree.Keypath to %v", typ)
		}
		val = reflect.New(typ).Elem()
		reflect.Copy(val.Slice(0, val.Len()), reflect.ValueOf([]byte(keypath)))

	default:
		return reflect.Value{}, errors.Errorf("could not convert tree.Keypath to %v", typ)
	}

	if val.Type() != typ {
		if val.Type().ConvertibleTo(typ) {
			return val.Convert(typ), nil
		}
		return reflect.Value{}, errors.Errorf("could not convert %v to %v", val.Type(), typ)
	}
	return val, nil
}

var (
	int64Type  = reflect.TypeOf(int64(0))
	uint64Type = reflect.TypeOf(uint64(0))
	stringType = reflect.TypeOf("")
	bytesType  = reflect.TypeOf([]byte(nil))
)

func convertToKeypath(val reflect.Value) (Keypath, error) {
	switch val.Kind() {
	case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
		i := val.Convert(uint64Type).Interface().(uint64)
		return Keypath(strconv.FormatUint(i, 10)), nil

	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		i := val.Convert(int64Type).Interface().(int64)
		return Keypath(strconv.FormatInt(i, 10)), nil

	case reflect.String:
		return Keypath(val.Convert(stringType).Interface().(string)), nil

	default:
		return nil, errors.Errorf("could not convert %v to tree.Keypath", val.Type())
	}
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

		if _, isNode := current.val.(Node); isNode {
			// Implementations of Node handle setting the contents of other Nodes
			// in implementation-defined ways, so we shouldn't attempt to do anything
			// here. See MemoryNode#setNode and DBNode#setNode for examples.
			continue

		} else if asMap, isMap := current.val.(map[string]interface{}); isMap {
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

		} else {
			rval := reflect.ValueOf(current.val)
			kind := rval.Kind()
			switch {
			case kind == reflect.Struct || (kind == reflect.Ptr && rval.Type().Elem().Kind() == reflect.Struct):
				z := structomancer.NewWithType(rval.Type(), StructTag)
				for _, fieldName := range z.FieldNames() {
					if fieldName == "-" {
						continue
					}
					rval, err := z.GetFieldValueV(rval, fieldName)
					if err != nil {
						return err
					}
					stack = append(stack, item{
						val:     rval.Interface(),
						keypath: current.keypath.Push(Keypath(fieldName)),
					})
				}

			case kind == reflect.Map:
				iter := rval.MapRange()
				for iter.Next() {
					keypath, err := convertToKeypath(iter.Key())
					if err != nil {
						return err
					}
					stack = append(stack, item{
						val:     iter.Value().Interface(),
						keypath: current.keypath.Push(keypath),
					})
				}

			case kind == reflect.Slice:
				// Special case []byte values -- because we can store them
				// directly, there's no need to encode them as normal slices
				if rval.Type().Elem().Kind() == reflect.Uint8 {
					continue
				}

				for i := 0; i < rval.Len(); i++ {
					stack = append(stack, item{
						val:     rval.Index(i).Interface(),
						keypath: current.keypath.Push(EncodeSliceIndex(uint64(i))),
					})
				}

			case kind == reflect.Array:
				// Special case [XX]byte values -- because we can store them
				// directly, there's no need to encode them as normal slices
				if rval.Type().Elem().Kind() == reflect.Uint8 {
					continue
				}

				for i := 0; i < rval.Len(); i++ {
					stack = append(stack, item{
						val:     rval.Index(i).Interface(),
						keypath: current.keypath.Push(EncodeSliceIndex(uint64(i))),
					})
				}
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

func debugPrint(inFormat string, args ...interface{}) {
	fmt.Printf(inFormat, args...)
}
