package nelson_test

import (
	"io"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/state"
)

type resolverMock struct {
	stateURIs  map[string]state.Node
	blobReader io.ReadCloser
	blobLength int64
}

func (m *resolverMock) StateAtVersion(stateURI string, version *state.Version) (state.Node, error) {
	state, exists := m.stateURIs[stateURI]
	if !exists {
		return nil, errors.Err404
	}
	return state, nil
}

func (m *resolverMock) BlobReader(blobID blob.ID) (io.ReadCloser, int64, error) {
	return m.blobReader, m.blobLength, nil
}
