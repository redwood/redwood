package nelson

import (
	"encoding/json"
	goerrors "errors"
	"io"
	"os"
	"strings"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/state"
)

type Frame struct {
	state.Node
	childmostNode state.Node
	contentType   string
	contentLength int64
	overrideValue interface{} // This is currently only used when a NelSON frame resolves to a blob, and we want to open that blob for the caller.  It will contain an io.ReadCloser.
	fullyResolved bool
	err           error
}

var (
	ValueKey         = state.Keypath("value")
	ContentTypeKey   = state.Keypath("Content-Type")
	ContentLengthKey = state.Keypath("Content-Length")
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

func (frame *Frame) Value(keypath state.Keypath, rng *state.Range) (interface{}, bool, error) {
	// @@TODO: how do we handle overrideValue if there's a keypath/range?
	if frame.overrideValue != nil {
		return frame.overrideValue, true, nil
	}
	return frame.Node.Value(keypath, rng)
}

func (frame *Frame) NodeAt(keypath state.Keypath, rng *state.Range) state.Node {
	if len(keypath) == 0 && rng == nil {
		return frame
	}
	return frame.Node.NodeAt(keypath, rng)
}

func (frame *Frame) ParentNodeFor(keypath state.Keypath) (state.Node, state.Keypath) {
	// This is necessary -- otherwise, fetching a Frame node will actually
	// return the underlying state.Node
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

func (frame *Frame) ValueNode() state.Node {
	return frame.Node
}

func (frame *Frame) DebugPrint(printFn func(inFormat string, args ...interface{}), newlines bool, indentLevel int) {
	if newlines {
		oldPrintFn := printFn
		printFn = func(inFormat string, args ...interface{}) { oldPrintFn(inFormat+"\n", args...) }
	}

	indent := strings.Repeat(" ", 4*indentLevel)

	printFn(indent + "NelSON Frame {")
	frame.Node.DebugPrint(printFn, false, indentLevel+1)
	printFn(indent + "}")
}

type StateResolver interface {
	StateAtVersion(stateURI string, version *state.Version) (state.Node, error)
}

type BlobResolver interface {
	BlobReader(blobID blob.ID) (io.ReadCloser, int64, error)
}

// Drills down to the provided keypath, resolving links as necessary. If the
// keypath resolves to a NelSON frame, the frame is resolved and returned.
// Otherwise, a regular state.Node is returned.
func Seek(
	node state.Node,
	keypath state.Keypath,
	stateResolver StateResolver,
	blobResolver BlobResolver,
) (_ state.Node, exists bool, _ error) {
	for len(keypath) > 0 {
		frameNode, nonFrameNode, remainingKeypath, err := DrillDownUntilFrame(node, keypath)
		if err != nil {
			return nil, false, err
		}
		keypath = remainingKeypath

		if frameNode == nil && nonFrameNode == nil {
			return nil, false, nil
		} else if frameNode == nil && nonFrameNode != nil {
			// ONLY can happen if len(keypath) == 0
			if len(keypath) != 0 {
				panic("nooooo")
			}
			return nonFrameNode, true, nil
		}

		// We have a frame
		frame, nonFrameNode, remainingKeypath, err := CollapseFrame(frameNode, keypath, stateResolver, blobResolver)
		if err != nil {
			return nil, false, err
		}
		keypath = remainingKeypath
		node = frame.Node

		if len(keypath) == 0 {
			return frame, true, nil
		}
	}
	return node, true, nil
}

// Given a state.Node, DrillDownUntilFrame will return the first NelSON frame
// encountered along the provided keypath.
func DrillDownUntilFrame(
	node state.Node,
	keypath state.Keypath,
) (frameNode, nonFrameNode state.Node, remaining state.Keypath, _ error) {
	for {
		isNelSONFrame, err := node.Exists(ValueKey)
		if err != nil {
			return nil, nil, nil, err
		}

		if isNelSONFrame {
			return node, nil, keypath, nil

		} else if len(keypath) == 0 {
			return nil, node, nil, nil
		}

		nodeExists, err := node.Exists(keypath.Part(0))
		if err != nil {
			return nil, nil, nil, err
		} else if !nodeExists {
			return nil, nil, nil, nil
		}

		node = node.NodeAt(keypath.Part(0), nil)
		_, keypath = keypath.Shift()
	}
}

// Given a regular state.Node representing a NelSON frame, CollapseFrame resolves
// that frame to a *Frame. If a state.Node is provided that does not represent a
// frame, the function will panic.
func CollapseFrame(
	node state.Node,
	keypath state.Keypath,
	stateResolver StateResolver,
	blobResolver BlobResolver,
) (*Frame, state.Node, state.Keypath, error) {
	exists, err := node.Exists(ContentTypeKey)
	if err != nil {
		return nil, nil, nil, err
	} else if !exists {
		panic("CollapseFrame was not passed a state.Node representing a NelSON frame")
	}

	frame := &Frame{Node: node}

	var contentType string
	for {
		frame.Node = node

		exists, err := node.Exists(ContentTypeKey)
		if err != nil {
			return nil, nil, nil, err
		} else if !exists {
			return frame, node, keypath, nil
		}

		thisContentType, _, err := node.StringValue(ContentTypeKey)
		if err != nil && errors.Cause(err) != errors.Err404 {
			return nil, nil, nil, err
		}

		// Simple frame
		if thisContentType != "link" {
			if contentType == "" {
				contentType = thisContentType
			}
			node = node.NodeAt(ValueKey, nil)
			continue
		}

		// Link frame
		linkStr, isString, err := node.StringValue(ValueKey)
		if err != nil && errors.Cause(err) != errors.Err404 {
			return nil, nil, nil, err
		} else if !isString {
			return nil, nil, nil, errors.Err404
		}

		linkType, linkValue := DetermineLinkType(linkStr)
		if linkType == LinkTypeBlob {
			if len(keypath) > 0 {
				return nil, nil, nil, errors.Err404
			}

			var blobID blob.ID
			err := blobID.UnmarshalText([]byte(linkValue))
			if err != nil {
				return nil, nil, nil, err
			}
			reader, contentLength, err := blobResolver.BlobReader(blobID)
			if err != nil && errors.Cause(err) != errors.Err404 {
				return nil, nil, nil, err
			}
			frame.overrideValue = reader
			frame.contentType = contentType
			frame.contentLength = contentLength
			frame.fullyResolved = errors.Cause(err) != errors.Err404
			return frame, nil, nil, nil

		} else if linkType == LinkTypeState {
			stateURI, linkedKeypath, version, err := ParseStateLink(linkValue)
			if err != nil {
				return nil, nil, nil, err
			}

			keypath = keypath.Unshift(linkedKeypath)

			root, err := stateResolver.StateAtVersion(stateURI, version)
			if errors.Cause(err) == errors.Err404 {
				frame.fullyResolved = false
				return frame, node, keypath, nil
			} else if err != nil {
				return nil, nil, nil, err
			}

			nodeExists, err := root.Exists(keypath.Part(0))
			if err != nil {
				return nil, nil, nil, err
			} else if !nodeExists {
				frame.fullyResolved = false
				return frame, node, keypath, nil
			}

			node = root.NodeAt(keypath.Part(0), nil)
			_, keypath = keypath.Shift()
			continue

		} else {
			return nil, nil, nil, errors.Errorf("unknown link type (%v)", linkStr)
		}
	}
	return frame, node, keypath, nil
}

// Given a state.Node, Resolve will recursively resolve all NelSON frames
// contained therein.
func Resolve(
	outerNode state.Node,
	stateResolver StateResolver,
	blobResolver BlobResolver,
) (state.Node, bool, error) {
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
				if err != nil && errors.Cause(err) != errors.Err404 {
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
						innerAnyMissing = resolveLink(frame, linkStr, stateResolver, blobResolver)
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
					if err != nil && errors.Cause(err) != errors.Err404 {
						return nil, false, err
					}
					if exists {
						frame.contentLength = contentLength
					}
				}

				// Innermost Content-Type wins
				if frame.contentType == "" {
					contentType, _, err := outerNode.StringValue(parentKeypath.Push(ContentTypeKey))
					if err != nil && errors.Cause(err) != errors.Err404 {
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

				// If we're at the state root, break
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

func resolveLink(frame *Frame, linkStr string, stateResolver StateResolver, blobResolver BlobResolver) (anyMissing bool) {
	linkType, linkValue := DetermineLinkType(linkStr)
	if linkType == LinkTypeBlob {
		var blobID blob.ID
		err := blobID.UnmarshalText([]byte(linkValue))
		if err != nil {
			frame.err = err
			return true
		}
		reader, contentLength, err := blobResolver.BlobReader(blobID)
		if goerrors.Is(err, os.ErrNotExist) {
			frame.err = errors.Err404
			return true
		} else if err != nil {
			frame.err = err
			return true
		} else {
			frame.overrideValue = reader
			frame.contentLength = contentLength
			return false
		}

	} else if linkType == LinkTypeState {
		stateURI, keypath, version, err := ParseStateLink(linkValue)
		if err != nil {
			frame.err = err
			return true
		}

		root, err := stateResolver.StateAtVersion(stateURI, version)
		if err != nil {
			frame.err = err
			return true
		}
		root, err = root.CopyToMemory(keypath, nil)
		if err != nil {
			frame.err = err
			return true
		}

		root, anyMissing, err := Resolve(root, stateResolver, blobResolver)
		if err != nil {
			frame.err = err
			return true
		}

		v, _, err := root.Value(nil, nil)
		if err != nil {
			frame.err = err
			return true
		}

		if asNelSON, isNelSON := v.(*Frame); isNelSON {
			frame.contentType = asNelSON.contentType
			frame.contentLength = asNelSON.contentLength
		}
		frame.Node = root
		frame.err = err
		return anyMissing

	} else {
		frame.err = errors.Errorf("unsupported link type: %v", linkStr)
		return true
	}
}

// ParseStateLink parses state links of the form "<state URI>[/<keypath>][@<version>]".
func ParseStateLink(linkValue string) (string, state.Keypath, *state.Version, error) {
	parts := strings.Split(linkValue, "/")
	var version *state.Version
	if i := strings.Index(parts[1], "@"); i >= 0 {
		vstr := parts[1][i:]
		v, err := state.VersionFromHex(vstr)
		if err != nil {
			return "", nil, nil, err
		}
		version = &v
		parts[1] = parts[1][:i]
	}
	// @@TODO: support range

	stateURI := strings.Join(parts[:2], "/")
	keypath := state.Keypath(linkValue[len(stateURI)+1:])
	return stateURI, keypath, version, nil
}
