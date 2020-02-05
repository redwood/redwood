package nelson

import (
	"encoding/json"
	goerrors "errors"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type Frame struct {
	tree.Node
	contentType   string
	contentLength int64
	overrideValue interface{} // This is currently only used when a NelSON frame resolves to a ref, and we want to open that ref for the caller.  It will contain an io.ReadCloser.
	fullyResolved bool
	err           error
}

var (
	ValueKey         = tree.Keypath("value")
	ContentTypeKey   = tree.Keypath("Content-Type")
	ContentLengthKey = tree.Keypath("Content-Length")
)

func (n *Frame) ContentType() (string, error) {
	if n.contentType != "" {
		return n.contentType, nil
	} else if !n.fullyResolved {
		return "application/json", nil
	}

	val, _, err := GetValueRecursive(n, nil, nil)
	if err != nil {
		return "", err
	}

	switch val.(type) {
	case nil, map[string]interface{}, []interface{}:
		return "application/json", nil
	case []byte, io.Reader:
		return "application/octet-stream", nil
	case string:
		return "text/plain", nil
	default:
		return "application/json", nil
	}
}

func (n *Frame) ContentLength() (int64, error) {
	return n.contentLength, nil
}

func (n *Frame) Value(keypath tree.Keypath, rng *tree.Range) (interface{}, bool, error) {
	// @@TODO: how do we handle overrideValue if there's a keypath/range?
	if n.overrideValue != nil {
		return n.overrideValue, true, nil
	}
	//return n.Node.Value(keypath, rng)
	return GetValueRecursive(n.Node, keypath, rng)
}

func (n *Frame) MarshalJSON() ([]byte, error) {
	val, exists, err := GetValueRecursive(n, nil, nil)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}
	return json.Marshal(val)
}

func (n *Frame) Err() error {
	return n.err
}

func (n *Frame) ValueNode() tree.Node {
	return n.Node
}

type ReferenceResolver interface {
	StateAtVersion(stateURI string, version *types.ID) (tree.Node, error)
	RefObjectReader(refHash types.Hash) (io.ReadCloser, int64, error)
}

func Resolve(frameNode tree.Node, refResolver ReferenceResolver) (tree.Node, bool, error) {
	var anyMissing bool

	iter := frameNode.DepthFirstIterator(nil, false, 0)
	defer iter.Close()
	for {
		node := iter.Next()
		if node == nil {
			break
		}

		keypath := node.Keypath().RelativeTo(frameNode.Keypath())

		parentKeypath, key := keypath.Pop()
		if key.Equals(ValueKey) {
			// This is a NelSON frame.

			copied, err := node.CopyToMemory(nil, nil)
			if err != nil {
				return nil, false, err
			}
			n := &Frame{Node: copied}

			// Before anything else, check for a link frame (transclusion).  If we find one,
			// don't set the Content-Type, just resolve the link.
			{
				contentType, _, err := frameNode.StringValue(parentKeypath.Push(ContentTypeKey))
				if err != nil {
					return nil, false, err
				}
				if contentType == "link" {
					linkStr, _, err := node.StringValue(nil)
					if err != nil {
						n.err = err
						n.fullyResolved = false
					} else {
						resolveLink(n, linkStr, refResolver)
					}
					anyMissing = anyMissing || !n.fullyResolved
				}
			}

			// Navigate up through the NelSON frame's ancestors checking for more NelSON frames.
		TravelUpwards:
			for {
				// Innermost Content-Length wins
				if n.contentLength == 0 {
					contentLength, exists, err := frameNode.IntValue(parentKeypath.Push(ContentLengthKey))
					if err != nil {
						return nil, false, err
					}
					if exists {
						n.contentLength = contentLength
					}
				}

				// Innermost Content-Type wins
				if n.contentType == "" {
					contentType, _, err := frameNode.StringValue(parentKeypath.Push(ContentTypeKey))
					if err != nil {
						return nil, false, err
					}
					// Enclosing (matryoshka-style) NelSON frames can't have a Content-Type of "link".  Only the innermost.
					if contentType != "link" {
						n.contentType = contentType
					}
				}

				newParentKeypath, newKey := parentKeypath.Pop()

				if !newKey.Equals(ValueKey) {
					if len(parentKeypath) == 0 {
						return n, false, nil

					} else {
						err := frameNode.Set(parentKeypath, nil, n)
						if err != nil {
							return nil, false, err
						}
						iter.(interface{ SeekTo(keypath tree.Keypath) }).SeekTo(parentKeypath)
					}

					break TravelUpwards
				}

				// If we're at the tree root, break
				if len(parentKeypath) == 0 {
					break TravelUpwards
				}

				parentKeypath = newParentKeypath
				key = newKey
			}
		}
	}
	return frameNode, anyMissing, nil
}

func resolveLink(n *Frame, linkStr string, refResolver ReferenceResolver) {
	linkType, linkValue := DetermineLinkType(linkStr)
	if linkType == LinkTypeRef {
		hash, err := types.HashFromHex(linkValue)
		if err != nil {
			n.err = err
			n.fullyResolved = false
			return
		}
		reader, contentLength, err := refResolver.RefObjectReader(hash)
		if goerrors.Is(err, os.ErrNotExist) {
			n.err = types.Err404
			n.fullyResolved = false
			return
		} else if err != nil {
			n.err = err
			n.fullyResolved = false
			return
		} else {
			n.overrideValue = reader
			n.contentLength = contentLength
			n.fullyResolved = true
			return
		}

	} else if linkType == LinkTypePath {
		parts := strings.Split(linkValue, "/")
		var version *types.ID
		if i := strings.Index(parts[1], "@"); i >= 0 {
			vstr := parts[1][i:]
			v, err := types.IDFromHex(vstr)
			if err != nil {
				n.err = err
				n.fullyResolved = false
				return
			}
			version = &v
			parts[1] = parts[1][:i]
		}
		// @@TODO: support range

		stateURI := strings.Join(parts[:2], "/")
		keypath := tree.Keypath(linkValue[len(stateURI)+1:])
		state, err := refResolver.StateAtVersion(stateURI, version)
		if err != nil {
			n.err = err
			n.fullyResolved = false
			return
		}
		state, err = state.CopyToMemory(keypath, nil)
		if err != nil {
			n.err = err
			n.fullyResolved = false
			return
		}

		state, anyMissing, err := Resolve(state, refResolver)
		if err != nil {
			n.err = err
			n.fullyResolved = false
			return
		}

		v, _, err := state.Value(nil, nil)
		if err != nil {
			n.err = err
			n.fullyResolved = false
			return
		}

		if asNelSON, isNelSON := v.(*Frame); isNelSON {
			n.contentType = asNelSON.contentType
			n.contentLength = asNelSON.contentLength
		}
		n.Node = state
		n.err = err
		n.fullyResolved = !anyMissing
		return

	} else {
		n.err = errors.Errorf("unsupported link type: %v", linkStr)
		n.fullyResolved = false
		return
	}
}
