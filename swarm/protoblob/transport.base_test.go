package protoblob_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"redwood.dev/blob"
	"redwood.dev/internal/testutils"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/protoblob/mocks"
	"redwood.dev/types"
)

func TestBaseBlobTransport_BlobManifestRequest(t *testing.T) {
	t.Parallel()

	var transport protoblob.BaseBlobTransport

	expectedPeerConn := new(mocks.BlobPeerConn)
	expectedBlobID := blob.ID{HashAlg: types.SHA3}

	callback1 := testutils.NewAwaiter()
	callback2 := testutils.NewAwaiter()

	transport.OnBlobManifestRequest(func(blobID blob.ID, peerConn protoblob.BlobPeerConn) {
		require.Equal(t, expectedBlobID, blobID)
		require.Equal(t, expectedPeerConn, peerConn)
		callback1.ItHappened()
	})

	transport.OnBlobManifestRequest(func(blobID blob.ID, peerConn protoblob.BlobPeerConn) {
		require.Equal(t, expectedBlobID, blobID)
		require.Equal(t, expectedPeerConn, peerConn)
		callback2.ItHappened()
	})

	transport.HandleBlobManifestRequest(expectedBlobID, expectedPeerConn)
	callback1.AwaitOrFail(t, 1*time.Second)
	callback2.AwaitOrFail(t, 1*time.Second)
}

func TestBaseBlobTransport_BlobChunkRequest(t *testing.T) {
	t.Parallel()

	var transport protoblob.BaseBlobTransport

	expectedPeerConn := new(mocks.BlobPeerConn)
	expectedBlobID := blob.ID{HashAlg: types.SHA3}

	callback1 := testutils.NewAwaiter()
	callback2 := testutils.NewAwaiter()

	transport.OnBlobChunkRequest(func(blobID blob.ID, peerConn protoblob.BlobPeerConn) {
		require.Equal(t, expectedBlobID, blobID)
		require.Equal(t, expectedPeerConn, peerConn)
		callback1.ItHappened()
	})

	transport.OnBlobChunkRequest(func(blobID blob.ID, peerConn protoblob.BlobPeerConn) {
		require.Equal(t, expectedBlobID, blobID)
		require.Equal(t, expectedPeerConn, peerConn)
		callback2.ItHappened()
	})

	transport.HandleBlobChunkRequest(expectedBlobID, expectedPeerConn)
	callback1.AwaitOrFail(t, 1*time.Second)
	callback2.AwaitOrFail(t, 1*time.Second)
}
