package utils

import (
	"encoding/json"
	"math/rand"
	"strconv"
)

func RandomBytes(length int) []byte {
	bs := make([]byte, length)
	rand.Read(bs)
	return bs
}

func RandomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func GetValue(x interface{}, keypath []string) (interface{}, bool) {
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

func PrettyJSON(x interface{}) string {
	j, _ := json.MarshalIndent(x, "", "    ")
	return string(j)
}
