package nelson

import (
	"encoding/json"
	goerrors "errors"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

var log = ctx.NewLogger("nelson")

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

func (frame *Frame) ContentType() (string, error) {
	if frame.contentType != "" {
		return frame.contentType, nil
	} else if !frame.fullyResolved {
		return "application/json", nil
	}

	val, _, err := GetValueRecursive(frame, nil, nil)
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

func (frame *Frame) ContentLength() (int64, error) {
	return frame.contentLength, nil
}

func (frame *Frame) DebugPrint() {
	log.Debug("NelSON Frame ----------------------------------------")
	frame.Node.DebugPrint()
	log.Debug("---------------------------------------------------")
}

func (frame *Frame) Value(keypath tree.Keypath, rng *tree.Range) (interface{}, bool, error) {
	// @@TODO: how do we handle overrideValue if there's a keypath/range?
	if frame.overrideValue != nil {
		return frame.overrideValue, true, nil
	}
	return frame.Node.Value(keypath, rng)
}

func (frame *Frame) NodeAt(keypath tree.Keypath, rng *tree.Range) tree.Node {
	if len(keypath) == 0 && rng == nil {
		return frame
	}
	return frame.Node.NodeAt(keypath, rng)
}

func (frame *Frame) ParentNodeFor(keypath tree.Keypath) (tree.Node, tree.Keypath) {
	// This is necessary -- otherwise, fetching a Frame node will actually
	// return the underlying tree.Node
	if len(keypath) == 0 {
		return frame, nil
	}
	parent, relKeypath := frame.Node.ParentNodeFor(keypath)
	if parent == frame.Node {
		return frame, relKeypath
	} else {
		return parent, relKeypath
	}
}

func (frame *Frame) MarshalJSON() ([]byte, error) {
	val, exists, err := GetValueRecursive(frame, nil, nil)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}
	return json.Marshal(val)
}

func (frame *Frame) Err() error {
	return frame.err
}

func (frame *Frame) ValueNode() tree.Node {
	return frame.Node
}

type ReferenceResolver interface {
	StateAtVersion(stateURI string, version *types.ID) (tree.Node, error)
	RefObjectReader(refHash types.Hash) (io.ReadCloser, int64, error)
}

func prettyJSON(x interface{}) string {
	j, _ := json.MarshalIndent(x, "", "    ")
	return string(j)
}

func Resolve(outerNode tree.Node, refResolver ReferenceResolver) (tree.Node, bool, error) {
	var anyMissing bool

	iter := outerNode.DepthFirstIterator(nil, false, 0)
	defer iter.Close()

	for iter.Rewind(); iter.Valid(); iter.Next() {
		node := iter.Node()
		keypath := node.Keypath().RelativeTo(outerNode.Keypath())

		parentKeypath, key := keypath.Pop()
		if key.Equals(ValueKey) {
			// This is a NelSON frame.

			copied, err := node.CopyToMemory(nil, nil)
			if err != nil {
				return nil, false, err
			}
			frame := &Frame{Node: copied}

			// Before anything else, check for a link frame (transclusion).  If we find one,
			// don't set the Content-Type, just resolve the link.
			{
				contentType, _, err := outerNode.StringValue(parentKeypath.Push(ContentTypeKey))
				if err != nil && errors.Cause(err) != types.Err404 {
					return nil, false, errors.Wrapf(err, "parentKeypath: %v", parentKeypath)
				}
				if contentType == "link" {
					var innerAnyMissing bool

					linkStr, _, err := frame.StringValue(nil)
					if err != nil {
						frame.err = err
						frame.fullyResolved = false
						anyMissing = true
					} else {
						innerAnyMissing = resolveLink(frame, linkStr, refResolver)
					}
					anyMissing = anyMissing || innerAnyMissing
					frame.fullyResolved = !anyMissing
				}
			}

			// Navigate up through the NelSON frame's ancestors checking for more NelSON frames.
		TravelUpwards:
			for {
				// Innermost Content-Length wins
				if frame.contentLength == 0 {
					contentLength, exists, err := outerNode.IntValue(parentKeypath.Push(ContentLengthKey))
					if err != nil && errors.Cause(err) != types.Err404 {
						return nil, false, err
					}
					if exists {
						frame.contentLength = contentLength
					}
				}

				// Innermost Content-Type wins
				if frame.contentType == "" {
					contentType, _, err := outerNode.StringValue(parentKeypath.Push(ContentTypeKey))
					if err != nil && errors.Cause(err) != types.Err404 {
						return nil, false, err
					}
					// Enclosing (matryoshka-style) NelSON frames can't have a Content-Type of "link".  Only the innermost.
					if contentType != "link" {
						frame.contentType = contentType
					}
				}

				nextParentKeypath, nextKey := parentKeypath.Pop()

				if !nextKey.Equals(ValueKey) {
					if len(parentKeypath) == 0 {
						frame.fullyResolved = !anyMissing
						return frame, anyMissing, nil

					} else {
						err := outerNode.Set(parentKeypath, nil, frame)
						if err != nil {
							return nil, false, err
						}
						iter.SeekTo(parentKeypath)
					}

					break TravelUpwards
				}

				// If we're at the tree root, break
				if len(parentKeypath) == 0 {
					break TravelUpwards
				}

				parentKeypath = nextParentKeypath
				key = nextKey
			}
		}
	}
	return outerNode, anyMissing, nil
}

func resolveLink(frame *Frame, linkStr string, refResolver ReferenceResolver) (anyMissing bool) {
	linkType, linkValue := DetermineLinkType(linkStr)
	if linkType == LinkTypeRef {
		hash, err := types.HashFromHex(linkValue)
		if err != nil {
			frame.err = err
			return true
		}
		reader, contentLength, err := refResolver.RefObjectReader(hash)
		if goerrors.Is(err, os.ErrNotExist) {
			frame.err = types.Err404
			return true
		} else if err != nil {
			frame.err = err
			return true
		} else {
			frame.overrideValue = reader
			frame.contentLength = contentLength
			return false
		}

	} else if linkType == LinkTypePath {
		parts := strings.Split(linkValue, "/")
		var version *types.ID
		if i := strings.Index(parts[1], "@"); i >= 0 {
			vstr := parts[1][i:]
			v, err := types.IDFromHex(vstr)
			if err != nil {
				frame.err = err
				return true
			}
			version = &v
			parts[1] = parts[1][:i]
		}
		// @@TODO: support range

		stateURI := strings.Join(parts[:2], "/")
		keypath := tree.Keypath(linkValue[len(stateURI)+1:])
		state, err := refResolver.StateAtVersion(stateURI, version)
		if err != nil {
			frame.err = err
			return true
		}
		state, err = state.CopyToMemory(keypath, nil)
		if err != nil {
			frame.err = err
			return true
		}

		state, anyMissing, err := Resolve(state, refResolver)
		if err != nil {
			frame.err = err
			return true
		}

		v, _, err := state.Value(nil, nil)
		if err != nil {
			frame.err = err
			return true
		}

		if asNelSON, isNelSON := v.(*Frame); isNelSON {
			frame.contentType = asNelSON.contentType
			frame.contentLength = asNelSON.contentLength
		}
		frame.Node = state
		frame.err = err
		return anyMissing

	} else {
		frame.err = errors.Errorf("unsupported link type: %v", linkStr)
		return true
	}
}
