package redwood

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func annotate(err *error, msg string, args ...interface{}) {
	if *err != nil {
		*err = errors.Wrapf(*err, msg, args...)
	}
}

func valueAtKeypath(m map[string]interface{}, keypath []string) (interface{}, bool) {
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
				kp := make([]string, len(current.keypath)+1)
				copy(kp, current.keypath)
				kp[len(kp)-1] = key
				stack = append(stack, item{
					val:     asMap[key],
					keypath: kp,
				})
			}
		}
	}
	return nil
}

func filterEmptyStrings(s []string) []string {
	var filtered []string
	for i := range s {
		if s[i] == "" {
			continue
		}
		filtered = append(filtered, s[i])
	}
	return filtered
}

func prettyJSON(val interface{}) string {
	j, _ := json.MarshalIndent(val, "", "    ")
	return string(j)
}

type M map[string]interface{}

func (m M) GetValue(keypath ...string) (interface{}, bool) {
	return valueAtKeypath(m, keypath)
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

func (m M) GetStringSlice(keypath ...string) ([]string, bool) {
	x, exists := valueAtKeypath(m, keypath)
	if !exists {
		return nil, false
	}
	if s, isSlice := x.([]string); isSlice {
		return s, true
	}
	return nil, false
}

func (m M) GetMap(keypath ...string) (map[string]interface{}, bool) {
	x, exists := valueAtKeypath(m, keypath)
	if !exists {
		return nil, false
	}
	if asMap, isMap := x.(map[string]interface{}); isMap {
		return asMap, true
	} else if asMap, isMap = x.(M); isMap {
		return (map[string]interface{})(asMap), true
	}
	return nil, false
}

func braidURLToHTTP(url string) string {
	if url[:6] == "braid:" {
		return "http:" + url[6:]
	}
	return url
}

func RedwoodConfigDirPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	redwoodConfigDir := filepath.Join(configDir, "redwood")

	err = os.MkdirAll(redwoodConfigDir, 0700)
	if err != nil {
		return "", err
	}

	return redwoodConfigDir, nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func PrettyJSON(x interface{}) string {
	j, _ := json.MarshalIndent(x, "", "    ")
	return string(j)
}
