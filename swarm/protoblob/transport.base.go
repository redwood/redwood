package protoblob

import (
	"sync"

	"redwood.dev/blob"
	"redwood.dev/types"
)

type BaseBlobTransport struct {
	muBlobManifestRequestCallbacks sync.RWMutex
	blobManifestRequestCallbacks   []func(blobID blob.ID, peer BlobPeerConn)
	muBlobChunkRequestCallbacks    sync.RWMutex
	blobChunkRequestCallbacks      []func(sha3 types.Hash, peer BlobPeerConn)
}

func (t *BaseBlobTransport) OnBlobManifestRequest(handler func(blobID blob.ID, peer BlobPeerConn)) {
	t.muBlobManifestRequestCallbacks.Lock()
	defer t.muBlobManifestRequestCallbacks.Unlock()
	t.blobManifestRequestCallbacks = append(t.blobManifestRequestCallbacks, handler)
}

func (t *BaseBlobTransport) HandleBlobManifestRequest(blobID blob.ID, peer BlobPeerConn) {
	t.muBlobManifestRequestCallbacks.RLock()
	defer t.muBlobManifestRequestCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.blobManifestRequestCallbacks))
	for _, handler := range t.blobManifestRequestCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			handler(blobID, peer)
		}()
	}
	wg.Wait()
}

func (t *BaseBlobTransport) OnBlobChunkRequest(handler func(sha3 types.Hash, peer BlobPeerConn)) {
	t.muBlobChunkRequestCallbacks.Lock()
	defer t.muBlobChunkRequestCallbacks.Unlock()
	t.blobChunkRequestCallbacks = append(t.blobChunkRequestCallbacks, handler)
}

func (t *BaseBlobTransport) HandleBlobChunkRequest(sha3 types.Hash, peer BlobPeerConn) {
	t.muBlobChunkRequestCallbacks.RLock()
	defer t.muBlobChunkRequestCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.blobChunkRequestCallbacks))
	for _, handler := range t.blobChunkRequestCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			handler(sha3, peer)
		}()
	}
	wg.Wait()
}
