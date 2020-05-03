package nelson

import (
	"io"

	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type refResolverMock struct {
	stateURIs       map[string]tree.Node
	refObjectReader io.ReadCloser
	refObjectLength int64
}

func (m *refResolverMock) StateAtVersion(stateURI string, version *types.ID) (tree.Node, error) {
	state, exists := m.stateURIs[stateURI]
	if !exists {
		return nil, types.Err404
	}
	return state, nil
}

func (m *refResolverMock) RefObjectReader(refHash types.Hash) (io.ReadCloser, int64, error) {
	return m.refObjectReader, m.refObjectLength, nil
}
