package redwood

import (
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
