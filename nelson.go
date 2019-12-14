package redwood

import (
	"bytes"
	"encoding/json"
	goerrors "errors"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
)

type NelSON struct {
	frame         interface{}
	contentType   string
	contentLength int64
	value         interface{}
	fullyResolved bool
	err           error
}

func (n NelSON) ContentType() string {
	if n.contentType != "" {
		return n.contentType
	} else if !n.fullyResolved {
		return "application/json"
	}

	switch n.value.(type) {
	case nil, map[string]interface{}, []interface{}:
		return "application/json"
	case []byte, io.Reader:
		return "application/octet-stream"
	case string:
		return "text/plain"
	default:
		return "application/json"
	}
}

func (n NelSON) ContentLength() int64 {
	return n.contentLength
}

func (n NelSON) Frame() interface{} {
	return n.frame
}

func (n NelSON) Value() interface{} {
	return n.value
}

func (n NelSON) ValueReader() (io.ReadCloser, error) {
	if n.fullyResolved == false || n.ContentType() == "application/json" {
		bs, err := json.Marshal(n.value)
		return ioutil.NopCloser(bytes.NewBuffer(bs)), err
	}

	readCloser, ok := GetReadCloser(n.value)
	if !ok {
		return nil, errors.Errorf("value can't have a Reader (%T)", n.value)
	}
	return readCloser, nil
}

func (n NelSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.value)
}

func (m *metacontroller) ResolveNelSON(frame interface{}) *NelSON {
	n := &NelSON{frame: frame, value: frame}
Loop:
	for {
		if n.value == nil {
			n.fullyResolved = false
			n.err = errors.New("got nil value")
			return n
		}

		switch v := n.value.(type) {
		case []byte, string, io.Reader:
			n.fullyResolved = true
			return n

		case map[string]interface{}:
			maybeContentLength, exists := getInt(v, []string{"Content-Length"})
			if exists {
				n.contentLength = int64(maybeContentLength)
			}

			maybeContentType, exists := getString(v, []string{"Content-Type"})
			if !exists {
				n.fullyResolved = true
				return n

			} else if exists && maybeContentType != "link" {
				n.contentType = maybeContentType
				n.value, _ = getValue(v, []string{"value"})

			} else if maybeContentType == "link" {
				linkStr, exists := getString(v, []string{"value"})
				if !exists {
					n.fullyResolved = false
					n.err = errors.New("link is missing value field")
					return n
				}

				linkType, linkValue := DetermineLinkType(linkStr)
				if linkType == LinkTypeRef {
					hash, err := HashFromHex(linkValue)
					if err != nil {
						n.fullyResolved = false
						n.err = err
						return n
					}
					reader, contentLength, err := m.refStore.Object(hash)
					if goerrors.Is(err, os.ErrNotExist) {
						n.fullyResolved = false
						n.err = err
						return n
					} else if err != nil {
						n.fullyResolved = false
						n.err = err
						return n
					} else {
						n.value = reader
						n.contentLength = contentLength
						n.fullyResolved = true
						return n
					}

				} else if linkType == LinkTypePath {
					parts := strings.Split(linkValue, "/")
					var version *ID
					if i := strings.Index(parts[1], "@"); i >= 0 {
						vstr := parts[1][i:]
						v, err := IDFromHex(vstr)
						if err != nil {
							n.fullyResolved = false
							n.err = err
							return n
						}
						version = &v
						parts[1] = parts[1][:i]
					}
					stateURI := strings.Join(parts[:2], "/")
					keypath := parts[2:]
					state, _, err := m.State(stateURI, keypath, version)
					if err != nil {
						n.fullyResolved = false
						n.err = err
						return n
					}
					n.value = state
					continue Loop

				} else {
					n.fullyResolved = false
					n.err = errors.Errorf("unsupported link type: %v", linkStr)
					return n
				}
			}

		default:
			n.fullyResolved = false
			n.err = errors.New("unsupported NelSON type")
			return n
		}
	}
	return nil
}
