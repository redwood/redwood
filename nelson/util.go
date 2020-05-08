package nelson

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

func Unwrap(node tree.Node) (tree.Node, tree.Keypath, error) {
	current := node
	currentKeypath := node.Keypath()
	for {
		switch n := current.(type) {
		case *Frame:
			current = n.Node
		case tree.Node:
			exists, err := n.Exists(ValueKey)
			if err != nil {
				// @@TODO: ??
				return nil, nil, err
			} else if !exists {
				return n, currentKeypath, nil
			}
			current = n.NodeAt(ValueKey, nil)
			_, currentKeypath = currentKeypath.Shift()
		}
	}
	panic("unreachable")
}

func GetValueRecursive(val interface{}, keypath tree.Keypath, rng *tree.Range) (interface{}, bool, error) {
	current := val
	var exists bool
	var err error
	for {
		if x, is := current.(tree.Node); is {
			current, exists, err = x.Value(keypath, rng)
			if err != nil {
				return nil, false, err
			} else if !exists {
				return nil, false, nil
			}
			keypath = nil
			rng = nil

		} else {
			if keypath == nil && rng == nil {
				return current, true, nil
			} else {
				return nil, false, nil
			}
		}
	}
}

func GetReadCloser(val interface{}) (io.ReadCloser, bool) {
	switch v := val.(type) {
	case string:
		return ioutil.NopCloser(bytes.NewBufferString(v)), true
	case []byte:
		return ioutil.NopCloser(bytes.NewBuffer(v)), true
	case io.ReadCloser:
		return v, true
	case io.Reader:
		return ioutil.NopCloser(v), true
	//case *Frame:
	//    rc, exists, err := v.ValueReader(nil, nil)
	//    if err != nil {
	//        return nil, false
	//    } else if !exists {
	//        return nil, false
	//    }
	//    return rc, true
	default:
		//var buf bytes.Buffer
		//json.NewEncoder(buf).Encode(val)
		//return buf, true
		return nil, false
	}
}

type ContentTyper interface {
	ContentType() (string, error)
}

type ContentLengther interface {
	ContentLength() (int64, error)
}

func GetContentType(val interface{}) (string, error) {
	switch v := val.(type) {
	case ContentTyper:
		return v.ContentType()

	case tree.Node:
		contentType, exists, err := GetValueRecursive(v, ContentTypeKey, nil)
		if err != nil && errors.Cause(err) == types.Err404 {
			return "application/json", nil
		} else if err != nil {
			return "", err
		}
		if s, isStr := contentType.(string); exists && isStr {
			return s, nil
		}
		return "application/json", nil

	default:
		return "application/json", nil
	}
}

func GetContentLength(val interface{}) (int64, error) {
	switch v := val.(type) {
	case ContentLengther:
		return v.ContentLength()

	case tree.Node:
		contentLength, exists, err := GetValueRecursive(v, ContentLengthKey, nil)
		if err != nil {
			return 0, err
		}
		if i, isInt := contentLength.(int64); exists && isInt {
			return i, nil
		}
		return 0, nil

	default:
		return 0, nil
	}
}

type LinkType int

const (
	LinkTypeUnknown LinkType = iota
	LinkTypeRef
	LinkTypeState
	LinkTypeURL // @@TODO
)

func DetermineLinkType(linkStr string) (LinkType, string) {
	if strings.HasPrefix(linkStr, "ref:") {
		return LinkTypeRef, linkStr[len("ref:"):]
	} else if strings.HasPrefix(linkStr, "state:") {
		return LinkTypeState, linkStr[len("state:"):]
	}
	return LinkTypeUnknown, linkStr
}
