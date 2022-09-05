package tree

import (
	"redwood.dev/state"
)

type VersionedDBTree struct {
	tree     *state.DBTree
	stateURI StateURI
}

func (t *VersionedDBTree) StateAtVersion(version *state.Version, mutable bool) *state.DBNode {
	if version == nil {
		version = &CurrentVersion
	}
	prefix := make([]byte, len(t.stateURI)+1+len(state.Version{})+1)
	p1 := []byte(t.stateURI + ":")
	p2 := append(version.Bytes(), ':')
	copy(prefix, p1)
	copy(prefix[len(p1):], p2)
	return t.tree.StateWithPrefix(prefix, mutable)
}

var (
	CurrentVersion = state.Version{}
)
