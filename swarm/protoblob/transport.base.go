package protoblob

import (
	"sync"

	"redwood.dev/blob"
)

type BaseBlobTransport struct {
	muBlobRequestCallbacks sync.RWMutex
	blobRequestCallbacks   []func(blobID blob.ID, peer BlobPeerConn)
}

func (t *BaseBlobTransport) OnBlobRequest(handler func(blobID blob.ID, peer BlobPeerConn)) {
	t.muBlobRequestCallbacks.Lock()
	defer t.muBlobRequestCallbacks.Unlock()
	t.blobRequestCallbacks = append(t.blobRequestCallbacks, handler)
}

func (t *BaseBlobTransport) HandleBlobRequest(blobID blob.ID, peer BlobPeerConn) {
	t.muBlobRequestCallbacks.RLock()
	defer t.muBlobRequestCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.blobRequestCallbacks))
	for _, handler := range t.blobRequestCallbacks {
		go func() {
			defer wg.Done()
			handler(blobID, peer)
		}()
	}
	wg.Wait()
}
