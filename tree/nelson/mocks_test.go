package nelson_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/url"
	"testing"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/types"
)

type resolverMock struct {
	stateResources map[string]stateResourceMock
	blobResources  map[blob.ID]blobResourceMock
	httpResources  map[url.URL]httpResourceMock
}

type stateResolverMock resolverMock
type blobResolverMock resolverMock
type httpResolverMock resolverMock

type stateResourceMock struct {
	node state.Node
	err  error
}

type blobResourceMock struct {
	reader   io.ReadCloser
	manifest blob.Manifest
	err      error
}

type httpResourceMock struct {
	contentType   string
	contentLength int64
	content       []byte
	err           error
}

func (m *stateResolverMock) addStateResource(stateURI string, node state.Node, err error) {
	if m.stateResources == nil {
		m.stateResources = make(map[string]stateResourceMock)
	}
	m.stateResources[stateURI] = stateResourceMock{node, err}
}

func (m *blobResolverMock) addBlobResource(blobID blob.ID, reader io.ReadCloser, manifest blob.Manifest, err error) {
	if m.blobResources == nil {
		m.blobResources = make(map[blob.ID]blobResourceMock)
	}
	m.blobResources[blobID] = blobResourceMock{reader, manifest, err}
}

func (m *httpResolverMock) addHTTPResource(u url.URL, contentType string, contentLength int64, content []byte, err error) {
	if m.httpResources == nil {
		m.httpResources = make(map[url.URL]httpResourceMock)
	}
	m.httpResources[u] = httpResourceMock{contentType, contentLength, content, err}
}

func (m *stateResolverMock) StateAtVersion(stateURI string, version *state.Version) (state.Node, error) {
	state, exists := m.stateResources[stateURI]
	if !exists {
		return nil, errors.Err404
	}
	return state.node, state.err
}

func (m *blobResolverMock) Manifest(blobID blob.ID) (blob.Manifest, error) {
	b, exists := m.blobResources[blobID]
	if !exists {
		return blob.Manifest{}, errors.Err404
	}
	return b.manifest, b.err
}

func (m *blobResolverMock) HaveBlob(blobID blob.ID) (bool, error) {
	_, exists := m.blobResources[blobID]
	return exists, nil
}

func (m *blobResolverMock) BlobReader(blobID blob.ID, rng *types.Range) (io.ReadCloser, int64, error) {
	blob, exists := m.blobResources[blobID]
	if !exists {
		return nil, 0, errors.Err404
	}
	return blob.reader, int64(blob.manifest.TotalSize), blob.err
}

func (m *httpResolverMock) Exists(url url.URL) (bool, error) {
	http, exists := m.httpResources[url]
	if !exists {
		return false, http.err
	} else if http.err != nil {
		return false, http.err
	}
	return true, nil
}

func (m *httpResolverMock) Get(url url.URL, byteRange *types.Range) (_ io.ReadCloser, contentType string, contentLength int64, _ error) {
	http, exists := m.httpResources[url]
	if !exists {
		return nil, "", 0, errors.Err404
	}
	return ioutil.NopCloser(bytes.NewReader(http.content)), http.contentType, http.contentLength, http.err
}

func mustURL(t *testing.T, s string) url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return *u
}
