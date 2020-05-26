package redwood

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	// "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

//var json = jsoniter.ConfigFastest
//var json = jsoniter.ConfigCompatibleWithStandardLibrary

func annotate(err *error, msg string, args ...interface{}) {
	if *err != nil {
		*err = errors.Wrapf(*err, msg, args...)
	}
}

func withStack(err *error) {
	if *err != nil {
		*err = errors.WithStack(*err)
	}
}

func combineErrors(errs []error) string {
	var errStrings []string
	for _, err := range errs {
		errStrings = append(errStrings, err.Error())
	}
	return strings.Join(errStrings, "\n")
}

func getValue(x interface{}, keypath []string) (interface{}, bool) {
	for i := 0; i < len(keypath); i++ {
		if asMap, isMap := x.(map[string]interface{}); isMap {
			var exists bool
			x, exists = asMap[keypath[i]]
			if !exists {
				return nil, false
			}

		} else if asSlice, isSlice := x.([]interface{}); isSlice {
			sliceIdx, err := strconv.ParseInt(keypath[i], 10, 64)
			if err != nil {
				return nil, false
			} else if sliceIdx > int64(len(asSlice)-1) {
				return nil, false
			}
			x = asSlice[sliceIdx]

		} else {
			return nil, false
		}
	}
	return x, true
}

func getString(m interface{}, keypath []string) (string, bool) {
	x, exists := getValue(m, keypath)
	if !exists {
		return "", false
	}
	if s, isString := x.(string); isString {
		return s, true
	}
	return "", false
}

func getInt(m interface{}, keypath []string) (int, bool) {
	x, exists := getValue(m, keypath)
	if !exists {
		return 0, false
	}
	if i, isInt := x.(int); isInt {
		return i, true
	}
	return 0, false
}

func getMap(m interface{}, keypath []string) (map[string]interface{}, bool) {
	x, exists := getValue(m, keypath)
	if !exists {
		return nil, false
	}
	if asMap, isMap := x.(map[string]interface{}); isMap {
		return asMap, true
	}
	return nil, false
}

func getSlice(m interface{}, keypath []string) ([]interface{}, bool) {
	x, exists := getValue(m, keypath)
	if !exists {
		return nil, false
	}
	if s, isSlice := x.([]interface{}); isSlice {
		return s, true
	}
	return nil, false
}

func getBool(m interface{}, keypath []string) (bool, bool) {
	x, exists := getValue(m, keypath)
	if !exists {
		return false, false
	}
	if b, isBool := x.(bool); isBool {
		return b, true
	}
	return false, false
}

func setValueAtKeypath(x interface{}, keypath []string, val interface{}, clobber bool) {
	if len(keypath) == 0 {
		panic("setValueAtKeypath: len(keypath) == 0")
	}

	var cur interface{} = x
	for i := 0; i < len(keypath)-1; i++ {
		key := keypath[i]

		if asMap, isMap := cur.(map[string]interface{}); isMap {
			var exists bool
			cur, exists = asMap[key]
			if !exists {
				if !clobber {
					return
				}
				asMap[key] = make(map[string]interface{})
				cur = asMap[key]
			}

		} else if asSlice, isSlice := cur.([]interface{}); isSlice {
			i, err := strconv.Atoi(key)
			if err != nil {
				panic(err)
			}
			cur = asSlice[i]
		} else {
			panic(fmt.Sprintf("setValueAtKeypath: bad type (%T)", cur))
		}
	}
	if asMap, isMap := cur.(map[string]interface{}); isMap {
		asMap[keypath[len(keypath)-1]] = val
	} else {
		panic(fmt.Sprintf("setValueAtKeypath: bad final type (%T)", cur))
	}
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

		if asMap, isMap := current.val.(map[string]interface{}); isMap {
			for key := range asMap {
				kp := make([]string, len(current.keypath)+1)
				copy(kp, current.keypath)
				kp[len(kp)-1] = key
				stack = append(stack, item{
					val:     asMap[key],
					keypath: kp,
				})
			}

		} else if asSlice, isSlice := current.val.([]interface{}); isSlice {
			for i := range asSlice {
				kp := make([]string, len(current.keypath)+1)
				copy(kp, current.keypath)
				kp[len(kp)-1] = strconv.Itoa(i)
				stack = append(stack, item{
					val:     asSlice[i],
					keypath: kp,
				})
			}
		}
	}
	return nil
}

func mapTree(tree interface{}, fn func(keypath []string, val interface{}) (interface{}, error)) (interface{}, error) {
	type item struct {
		val     interface{}
		parent  interface{}
		keypath []string
	}

	stack := []item{{val: tree, keypath: []string{}}}
	var current item
	var firstLoop = true

	for len(stack) > 0 {
		current = stack[0]
		stack = stack[1:]

		newVal, err := fn(current.keypath, current.val)
		if err != nil {
			return nil, err
		}

		if firstLoop {
			tree = newVal
			firstLoop = false
		}

		if asMap, isMap := current.parent.(map[string]interface{}); isMap {
			asMap[current.keypath[len(current.keypath)-1]] = newVal
		} else if asSlice, isSlice := current.parent.([]interface{}); isSlice {
			i, err := strconv.Atoi(current.keypath[len(current.keypath)-1])
			if err != nil {
				return nil, errors.WithStack(err)
			}
			asSlice[i] = newVal
		}

		if asMap, isMap := newVal.(map[string]interface{}); isMap {
			for key := range asMap {
				kp := make([]string, len(current.keypath)+1)
				copy(kp, current.keypath)
				kp[len(kp)-1] = key
				stack = append(stack, item{
					val:     asMap[key],
					keypath: kp,
					parent:  newVal,
				})
			}

		} else if asSlice, isSlice := newVal.([]interface{}); isSlice {
			for i := range asSlice {
				kp := make([]string, len(current.keypath)+1)
				copy(kp, current.keypath)
				kp[len(kp)-1] = strconv.Itoa(i)
				stack = append(stack, item{
					val:     asSlice[i],
					keypath: kp,
					parent:  newVal,
				})
			}
		}
	}
	return tree, nil
}

func walkContentTypes(state interface{}, contentTypes []string, fn func(contentType string, keypath []string, val map[string]interface{}) error) error {
	return walkTree(state, func(keypath []string, val interface{}) error {
		asMap, isMap := val.(map[string]interface{})
		if !isMap {
			return nil
		}

		for _, ct := range contentTypes {
			contentType, exists := getString(asMap, []string{"Content-Type"})
			if !exists || contentType != ct {
				continue
			}
			return fn(contentType, keypath, asMap)
		}
		return nil
	})
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

// @@TODO: everything about this is horrible
func DeepCopyJSValue(val interface{}) interface{} {
	bs, err := json.Marshal(val)
	if err != nil {
		panic(err)
	}
	var copied interface{}
	err = json.Unmarshal(bs, &copied)
	if err != nil {
		panic(err)
	}
	return copied
}

type StringSet map[string]struct{}

func NewStringSet(vals []string) StringSet {
	set := map[string]struct{}{}
	for _, val := range vals {
		set[val] = struct{}{}
	}
	return set
}

func (s StringSet) Add(val string) {
	s[val] = struct{}{}
}

func (s StringSet) Remove(val string) {
	delete(s, val)
}

func (s StringSet) Any() string {
	for x := range s {
		return x
	}
	return ""
}

func (s StringSet) Slice() []string {
	var slice []string
	for x := range s {
		slice = append(slice, x)
	}
	return slice
}

func (s StringSet) Copy() StringSet {
	set := map[string]struct{}{}
	for val := range s {
		set[val] = struct{}{}
	}
	return set
}

func SniffContentType(filename string, data io.Reader) (string, error) {
	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	_, err := data.Read(buffer)
	if err != nil {
		return "", err
	}

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)

	// If we got an ambiguous result, check the file extension
	if contentType == "application/octet-stream" {
		contentType = GuessContentTypeFromFilename(filename)
	}
	return contentType, nil
}

func GuessContentTypeFromFilename(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		ext := strings.ToLower(parts[len(parts)-1])
		switch ext {
		case "txt":
			return "text/plain"
		case "html":
			return "text/html"
		case "js":
			return "application/js"
		case "json":
			return "application/json"
		case "png":
			return "image/png"
		case "jpg", "jpeg":
			return "image/jpeg"
		}
	}
	return "application/octet-stream"
}
