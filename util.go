package redwood

import (
	"encoding/json"

	"github.com/pkg/errors"
)

func annotate(err *error, msg string, args ...interface{}) {
	if *err != nil {
		*err = errors.Wrapf(*err, msg, args...)
	}
}

func valueAtKeypath(maybeMap interface{}, keypath []string) (interface{}, bool) {
	m, isMap := maybeMap.(map[string]interface{})
	if !isMap {
		if len(keypath) > 0 {
			return nil, false
		}
		return maybeMap, true
	}

	var cur interface{} = m
	for i := 0; i < len(keypath); i++ {
		var exists bool
		cur, exists = m[keypath[i]]
		if !exists {
			return nil, false
		}

		if i < len(keypath)-1 {
			var isMap bool
			m, isMap = cur.(map[string]interface{})
			if !isMap {
				return nil, false
			}
		}
	}
	return cur, true
}

func walkTree(tree interface{}, fn func(keypath []string, val interface{}) error) error {
	type item struct {
		val     interface{}
		keypath []string
	}

	stack := []item{{val: tree, keypath: []string{}}}
	var current item

	for len(stack) > 0 {
		current = stack[0]
		stack = stack[1:]

		err := fn(current.keypath, current.val)
		if err != nil {
			return err
		}

		asMap, isMap := current.val.(map[string]interface{})
		if isMap {
			for key := range asMap {
				stack = append(stack, item{
					val:     asMap[key],
					keypath: append(current.keypath, key),
				})
			}
		}
	}
	return nil
}

func prettyJSON(val interface{}) string {
	j, _ := json.MarshalIndent(val, "", "    ")
	return string(j)
}

type M map[string]interface{}

func (m M) GetValue(keypath ...string) (interface{}, bool) {
	return valueAtKeypath(map[string]interface{}(m), keypath)
}

func (m M) GetString(keypath ...string) (string, bool) {
	x, exists := valueAtKeypath(m, keypath)
	if !exists {
		return "", false
	}
	if s, isString := x.(string); isString {
		return s, true
	}
	return "", false
}

func (m M) GetSlice(keypath ...string) ([]interface{}, bool) {
	x, exists := valueAtKeypath(m, keypath)
	if !exists {
		return nil, false
	}
	if s, isSlice := x.([]interface{}); isSlice {
		return s, true
	}
	return nil, false
}

func (m M) GetMap(keypath ...string) (map[string]interface{}, bool) {
	x, exists := valueAtKeypath(m, keypath)
	if !exists {
		return nil, false
	}
	if m, isMap := x.(map[string]interface{}); isMap {
		return m, true
	}
	return nil, false
}
