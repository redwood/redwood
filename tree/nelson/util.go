package nelson

import (
	"strings"

	"redwood.dev/errors"
	"redwood.dev/state"
)

func ContentTypeOf(node state.Node) (string, error) {
	switch node := node.(type) {
	case Node:
		return node.ContentType(), nil
	default:
		contentType, _, err := node.StringValue(ContentTypeKey)
		if err != nil {
			return "", err
		}
		return contentType, nil
	}
}

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

type LinkType int

const (
	LinkTypeInvalid LinkType = iota
	LinkTypeBlob
	LinkTypeState
	LinkTypeHTTP
)

func DetermineLinkType(linkStr string) (LinkType, string) {
	if strings.HasPrefix(linkStr, "blob:") {
		return LinkTypeBlob, linkStr[len("blob:"):]
	} else if strings.HasPrefix(linkStr, "ref:") {
		return LinkTypeBlob, linkStr[len("ref:"):]
	} else if strings.HasPrefix(linkStr, "state:") {
		return LinkTypeState, linkStr[len("state:"):]
	}
	return LinkTypeInvalid, linkStr
}

// ParseStateLink parses state links of the form "<state URI>[/<keypath>][@<version>]".
func ParseStateLink(linkValue string) (stateURI string, keypath state.Keypath, version *state.Version, _ error) {
	parts := strings.Split(linkValue, "/")
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

	stateURI = strings.Join(parts[:2], "/")
	keypath = state.Keypath(linkValue[len(stateURI)+1:])
	return stateURI, keypath, version, nil
}
