package nelson

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"

	"redwood.dev/errors"
	"redwood.dev/state"
)

// Recurses down through a series of Frame nodes until it finds a non-Frame node
// and returns it. If maxDepth is exceeded, errors.Err404 is returned.
func FirstNonFrameNode(node state.Node, maxDepth uint64) (state.Node, error) {
	current := node
	for i := uint64(0); i < maxDepth; i++ {
		switch n := current.(type) {
		case *Frame:
			current = n.Node
		case state.Node:
			exists, err := n.Exists(ValueKey)
			if err != nil {
				// @@TODO: ??
				return nil, err
			} else if !exists {
				return n, nil
			}
			current = n.NodeAt(ValueKey, nil)
		}
	}
	return nil, errors.Err404
}

func GetValueRecursive(val interface{}, keypath state.Keypath, rng *state.Range) (interface{}, bool, error) {
	current := val
	var exists bool
	var err error
	for {
		if x, is := current.(state.Node); is {
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

	case state.Node:
		contentType, exists, err := GetValueRecursive(v, ContentTypeKey, nil)
		if err != nil && errors.Cause(err) == errors.Err404 {
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

	case state.Node:
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
	LinkTypeBlob
	LinkTypeState
	LinkTypeURL // @@TODO
)

func DetermineLinkType(linkStr string) (LinkType, string) {
	if strings.HasPrefix(linkStr, "blob:") {
		return LinkTypeBlob, linkStr[len("blob:"):]
	} else if strings.HasPrefix(linkStr, "ref:") {
		return LinkTypeBlob, linkStr[len("ref:"):]
	} else if strings.HasPrefix(linkStr, "state:") {
		return LinkTypeState, linkStr[len("state:"):]
	}
	return LinkTypeUnknown, linkStr
}
