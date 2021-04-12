package nelson_test

import (
	"io"

	"redwood.dev/state"
	"redwood.dev/types"
)

type refResolverMock struct {
	stateURIs       map[string]state.Node
	refObjectReader io.ReadCloser
	refObjectLength int64
}

func (m *refResolverMock) StateAtVersion(stateURI string, version *types.ID) (state.Node, error) {
	state, exists := m.stateURIs[stateURI]
	if !exists {
		return nil, types.Err404
	}
	return state, nil
}

func (m *refResolverMock) RefObjectReader(refID types.RefID) (io.ReadCloser, int64, error) {
	return m.refObjectReader, m.refObjectLength, nil
}
