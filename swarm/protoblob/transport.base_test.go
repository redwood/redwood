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

func TestBaseBlobTransport_BlobRequest(t *testing.T) {
	t.Parallel()

	var transport protoblob.BaseBlobTransport

	expectedPeerConn := new(mocks.BlobPeerConn)
	expectedBlobID := blob.ID{HashAlg: types.SHA3}

	callback1 := testutils.NewAwaiter()
	callback2 := testutils.NewAwaiter()

	transport.OnBlobRequest(func(blobID blob.ID, peerConn protoblob.BlobPeerConn) {
		require.Equal(t, expectedBlobID, blobID)
		require.Equal(t, expectedPeerConn, peerConn)
		callback1.ItHappened()
	})

	transport.OnBlobRequest(func(blobID blob.ID, peerConn protoblob.BlobPeerConn) {
		require.Equal(t, expectedBlobID, blobID)
		require.Equal(t, expectedPeerConn, peerConn)
		callback2.ItHappened()
	})

	transport.HandleBlobRequest(expectedBlobID, expectedPeerConn)
	callback1.AwaitOrFail(t, 1*time.Second)
	callback2.AwaitOrFail(t, 1*time.Second)
}
