package nelson

import (
	"fmt"
	"io"
	"net/url"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/types"
)

type Resolver struct {
	stateResolver StateResolver
	blobResolver  BlobResolver
	httpResolver  HTTPResolver
}

func NewResolver(stateResolver StateResolver, blobResolver BlobResolver, httpResolver HTTPResolver) Resolver {
	return Resolver{stateResolver, blobResolver, httpResolver}
}

type StateResolver interface {
	StateAtVersion(stateURI string, version *state.Version) (state.Node, error)
}

type BlobResolver interface {
	HaveBlob(blobID blob.ID) (bool, error)
	Manifest(blobID blob.ID) (blob.Manifest, error)
	BlobReader(blobID blob.ID, byteRange *types.Range) (io.ReadCloser, int64, error)
}

type HTTPResolver interface {
	Metadata(url url.URL) (exists bool, contentType string, contentLength int64, err error)
	Get(url url.URL, byteRange *types.Range) (_ io.ReadCloser, contentType string, contentLength int64, _ error)
}

// Drills down to the provided keypath, resolving links as necessary. If the
// keypath resolves to a NelSON frame, the frame is returned as a nelson.Frame.
// Otherwise, a regular state.Node is returned.
func (r Resolver) Seek(node state.Node, keypath state.Keypath) (sought state.Node, exists bool, _ error) {
	for len(keypath) > 0 {
		frameNode, nonFrameNode, remainingKeypath, err := r.drillDownUntilFrame(node, keypath)
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
		frameNode, err = r.collapseBasicFrame(frameNode, keypath)
		if err != nil {
			return nil, false, err
		}
		node = frameNode
	}
	return node, true, nil
}

// Given a state.Node, DrillDownUntilFrame will return the first NelSON frame
// encountered along the provided keypath.
func (r Resolver) drillDownUntilFrame(
	node state.Node,
	keypath state.Keypath,
) (frameNode, nonFrameNode state.Node, remaining state.Keypath, _ error) {
	for {
		is, err := isNelSONFrame(node)
		if err != nil {
			return nil, nil, nil, err
		}

		if is {
			return node, nil, keypath, nil

		} else if len(keypath) == 0 {
			return nil, node, nil, nil
		}

		var key state.Keypath
		key, keypath = keypath.Shift()

		nodeExists, err := node.Exists(key)
		if err != nil {
			return nil, nil, nil, err
		} else if !nodeExists {
			return nil, nil, nil, errors.Err404
		}

		node = node.NodeAt(key, nil)
	}
}

// Given a regular state.Node representing a NelSON frame, `collapseBasicFrame`
// resolves that frame to a nelson.Frame. If a state.Node is provided that does not
// represent a frame, the function will panic.
func (r Resolver) collapseBasicFrame(
	node state.Node,
	keypath state.Keypath,
) (frame Frame, _ error) {
	// fmt.Println("collapse", keypath, utils.PrettyJSON(node))

	if frame, is := node.(Frame); is {
		return frame, nil
	}

	is, err := isNelSONFrame(node)
	if err != nil {
		return Frame{}, err
	} else if !is {
		panic("collapseBasicFrame was not passed a state.Node representing a NelSON frame")
	}

	var contentType string
	for {
		// fmt.Println("xyzzy NODE", utils.PrettyJSON(node))
		is, err := isNelSONFrame(node)
		if err != nil {
			return Frame{}, err
		} else if !is {
			frame := Frame{Node: node, contentType: contentType}
			return frame, nil
		}

		thisContentType, _, err := node.StringValue(ContentTypeKey)
		if err != nil && errors.Cause(err) != errors.Err404 {
			return Frame{}, err
		}

		// Simple frame
		if thisContentType != "link" {
			if thisContentType != "" {
				contentType = thisContentType
			}
			// `isNelSONFrame` proves that `ValueKey` exists
			node = node.NodeAt(ValueKey, nil)
			continue
		}

		// Link frame
		linkStr, isString, err := node.StringValue(ValueKey)
		if err != nil && errors.Cause(err) != errors.Err404 {
			return Frame{}, err
		} else if !isString {
			return Frame{}, errors.Err404
		}

		linkType, linkValue := DetermineLinkType(linkStr)

		switch linkType {
		case LinkTypeBlob:
			if len(keypath) > 0 {
				return Frame{}, errors.Err404
			}
			return Frame{Node: node, contentType: contentType, linkType: linkType, linkValue: linkValue}, nil

		case LinkTypeHTTP:
			if len(keypath) > 0 {
				return Frame{}, errors.Err404
			}
			return Frame{Node: node, contentType: contentType, linkType: linkType, linkValue: linkValue}, nil

		case LinkTypeState:
			stateURI, linkedKeypath, version, err := ParseStateLink(linkValue)
			if err != nil {
				return Frame{}, err
			}
			fmt.Println("xyzzy state link", stateURI, linkedKeypath)

			node.Close()
			root, err := r.stateResolver.StateAtVersion(stateURI, version)
			if err != nil {
				return Frame{}, err
			}

			exists, err := root.Exists(linkedKeypath)
			if err != nil {
				return Frame{}, err
			} else if !exists {
				return Frame{}, errors.Err404
			}

			node = root.NodeAt(linkedKeypath, nil)
			return Frame{Node: node, contentType: contentType, linkType: linkType, linkValue: linkValue}, nil

		default:
			return Frame{}, errors.Errorf("unknown link type (%v)", linkStr)
		}
	}
}

func isNelSONFrame(node state.Node) (bool, error) {
	return node.Exists(ValueKey)
}

// Given a state.Node, Resolve will recursively resolve all NelSON frames
// contained therein.
func (r Resolver) Resolve(stateNode state.Node) (resolved state.Node, anyMissing bool, err error) {
	if _, is := stateNode.(*state.DBNode); is {
		stateNode, err = stateNode.CopyToMemory(nil, nil)
		if err != nil {
			return
		}
	}

	stack := []state.Keypath{nil}

	resolved = stateNode

	for len(stack) > 0 {
		keypath := stack[0]
		stack = stack[1:]

		node := stateNode.NodeAt(keypath, nil)

		switch typedNode := node.(type) {
		case Frame:
			resolvedFrame, innerAnyMissing, err := r.resolveFrame(typedNode)
			if err != nil {
				return nil, false, err
			}
			anyMissing = anyMissing || innerAnyMissing

			if len(keypath) > 0 {
				parentKeypath, childKey := keypath.Pop()
				err = stateNode.NodeAt(parentKeypath, nil).Set(childKey, nil, resolvedFrame)
				if err != nil {
					return nil, false, err
				}
			} else {
				resolved = resolvedFrame
			}
			node = resolvedFrame

		case state.Node:
			is, err := isNelSONFrame(typedNode)
			if err != nil {
				return nil, false, err
			}
			if is {
				frame, err := r.collapseBasicFrame(typedNode, nil)
				if errors.Cause(err) == errors.Err404 {
					anyMissing = true
					continue
				} else if err != nil {
					return nil, false, err
				}

				// frame.DebugPrint(utils.PrintfDebugPrinter, true, 0)

				copied, err := frame.Node.CopyToMemory(nil, nil)
				if err != nil {
					return nil, false, err
				}
				frame.Node = copied

				resolvedFrame, innerAnyMissing, err := r.resolveFrame(frame)
				if err != nil {
					return nil, false, err
				}

				anyMissing = anyMissing || innerAnyMissing

				if len(keypath) > 0 {
					err = stateNode.Set(keypath, nil, resolvedFrame)
					if err != nil {
						return nil, false, err
					}
				} else {
					resolved = resolvedFrame
				}
				node = resolvedFrame
			}

		default:
			panic("no")
		}

		err := func() error {
			iter := node.ChildIterator(nil, false, 0)
			defer iter.Close()

			for iter.Rewind(); iter.Valid(); iter.Next() {
				nodeType, _, _, err := iter.Node().NodeInfo(nil)
				if err != nil {
					return err
				} else if nodeType != state.NodeTypeMap && nodeType != state.NodeTypeSlice {
					continue
				}
				childNode := iter.Node()
				stack = append(stack, childNode.Keypath())
			}
			return nil
		}()
		if err != nil {
			return nil, false, err
		}
	}
	return
}

func (r Resolver) resolveFrame(frame Frame) (resolved Node, anyMissing bool, _ error) {
	switch frame.linkType {
	case LinkTypeBlob:
		var blobID blob.ID
		err := blobID.UnmarshalText([]byte(frame.linkValue))
		if err != nil {
			return nil, false, err
		}
		have, err := r.blobResolver.HaveBlob(blobID)
		if err != nil {
			return nil, false, err
		}
		return BlobFrame{Frame: frame, resolver: r, blobID: blobID}, !have, nil

	case LinkTypeHTTP:
		u, err := url.Parse(frame.linkValue)
		if err != nil {
			return nil, false, err
		}
		exists, _, _, err := r.httpResolver.Metadata(*u)
		if err != nil {
			return nil, false, err
		}
		return HTTPFrame{Frame: frame, resolver: r, url: *u}, !exists, nil

	case LinkTypeState:
		node, err := frame.Node.CopyToMemory(nil, nil)
		if err != nil {
			return nil, false, err
		}

		resolved, innerAnyMissing, err := r.Resolve(node)
		if err != nil {
			return nil, false, err
		}

		switch resolved := resolved.(type) {
		case Frame:
			if resolved.contentType == "" {
				resolved.contentType = frame.contentType
			}
			return resolved, innerAnyMissing, nil

		case BlobFrame:
			if resolved.contentType == "" {
				resolved.contentType = frame.contentType
			}
			return resolved, innerAnyMissing, nil

		case HTTPFrame:
			if resolved.contentType == "" {
				resolved.contentType = frame.contentType
			}
			return resolved, innerAnyMissing, nil

		case state.Node:
			frame.Node = resolved
			return frame, innerAnyMissing, nil

		default:
			panic("no")
		}

	default:
		return frame, false, nil
	}
}
