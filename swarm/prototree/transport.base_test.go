package prototree_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"redwood.dev/internal/testutils"
	"redwood.dev/state"
	"redwood.dev/swarm/prototree"
	"redwood.dev/swarm/prototree/mocks"
	"redwood.dev/tree"
)

func TestBaseTreeTransport_TxReceived(t *testing.T) {
	t.Parallel()

	var transport prototree.BaseTreeTransport

	expectedTx := tree.Tx{ID: state.VersionFromString("foo bar")}
	expectedPeerConn := new(mocks.TreePeerConn)

	callback1 := testutils.NewAwaiter()
	callback2 := testutils.NewAwaiter()

	transport.OnTxReceived(func(tx tree.Tx, peerConn prototree.TreePeerConn) {
		require.Equal(t, expectedTx, tx)
		require.Equal(t, expectedPeerConn, peerConn)
		callback1.ItHappened()
	})

	transport.OnTxReceived(func(tx tree.Tx, peerConn prototree.TreePeerConn) {
		require.Equal(t, expectedTx, tx)
		require.Equal(t, expectedPeerConn, peerConn)
		callback2.ItHappened()
	})

	transport.HandleTxReceived(expectedTx, expectedPeerConn)
	callback1.AwaitOrFail(t, 1*time.Second)
	callback2.AwaitOrFail(t, 1*time.Second)
}

func TestBaseTreeTransport_AckReceived(t *testing.T) {
	t.Parallel()

	var transport prototree.BaseTreeTransport

	expectedStateURI := "foo.bar/blah"
	expectedTxID := state.RandomVersion()
	expectedPeerConn := new(mocks.TreePeerConn)

	callback1 := testutils.NewAwaiter()
	callback2 := testutils.NewAwaiter()

	transport.OnAckReceived(func(stateURI string, txID state.Version, peerConn prototree.TreePeerConn) {
		require.Equal(t, expectedStateURI, stateURI)
		require.Equal(t, expectedTxID, txID)
		require.Equal(t, expectedPeerConn, peerConn)
		callback1.ItHappened()
	})

	transport.OnAckReceived(func(stateURI string, txID state.Version, peerConn prototree.TreePeerConn) {
		require.Equal(t, expectedStateURI, stateURI)
		require.Equal(t, expectedTxID, txID)
		require.Equal(t, expectedPeerConn, peerConn)
		callback2.ItHappened()
	})

	transport.HandleAckReceived(expectedStateURI, expectedTxID, expectedPeerConn)
	callback1.AwaitOrFail(t, 1*time.Second)
	callback2.AwaitOrFail(t, 1*time.Second)
}

func TestBaseTreeTransport_WritableSubscriptionOpened(t *testing.T) {
	t.Parallel()

	var transport prototree.BaseTreeTransport

	expectedStateURI := "foo.bar/blah"
	expectedKeypath := state.Keypath("hello")
	expectedSubType := prototree.SubscriptionType_States
	expectedWriteSubImpl := new(mocks.WritableSubscriptionImpl)
	expectedFetchHistoryOpts := &prototree.FetchHistoryOpts{FromTxID: state.RandomVersion(), ToTxID: state.RandomVersion()}

	callback := testutils.NewAwaiter()

	transport.OnWritableSubscriptionOpened(func(req prototree.SubscriptionRequest, writeSubImplFactory prototree.WritableSubscriptionImplFactory) (<-chan struct{}, error) {
		writeSubImpl, err := writeSubImplFactory()
		require.NoError(t, err)
		require.Equal(t, expectedStateURI, req.StateURI)
		require.Equal(t, expectedKeypath, req.Keypath)
		require.Equal(t, expectedSubType, req.Type)
		require.Equal(t, expectedWriteSubImpl, writeSubImpl)
		require.Equal(t, expectedFetchHistoryOpts, req.FetchHistoryOpts)
		callback.ItHappened()
		return nil, nil
	})

	transport.HandleWritableSubscriptionOpened(prototree.SubscriptionRequest{
		StateURI:         expectedStateURI,
		Keypath:          expectedKeypath,
		Type:             expectedSubType,
		FetchHistoryOpts: expectedFetchHistoryOpts,
	}, func() (prototree.WritableSubscriptionImpl, error) {
		return expectedWriteSubImpl, nil
	})
	callback.AwaitOrFail(t, 1*time.Second)
}
