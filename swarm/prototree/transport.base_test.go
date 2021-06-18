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
	"redwood.dev/types"
)

func TestBaseTreeTransport_TxReceived(t *testing.T) {
	t.Parallel()

	var transport prototree.BaseTreeTransport

	expectedTx := tree.Tx{ID: types.IDFromString("foo bar")}
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
	expectedTxID := types.RandomID()
	expectedPeerConn := new(mocks.TreePeerConn)

	callback1 := testutils.NewAwaiter()
	callback2 := testutils.NewAwaiter()

	transport.OnAckReceived(func(stateURI string, txID types.ID, peerConn prototree.TreePeerConn) {
		require.Equal(t, expectedStateURI, stateURI)
		require.Equal(t, expectedTxID, txID)
		require.Equal(t, expectedPeerConn, peerConn)
		callback1.ItHappened()
	})

	transport.OnAckReceived(func(stateURI string, txID types.ID, peerConn prototree.TreePeerConn) {
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
	expectedFetchHistoryOpts := &prototree.FetchHistoryOpts{FromTxID: types.RandomID(), ToTxID: types.RandomID()}

	callback1 := testutils.NewAwaiter()
	callback2 := testutils.NewAwaiter()

	transport.OnWritableSubscriptionOpened(func(stateURI string, keypath state.Keypath, subType prototree.SubscriptionType, writeSubImpl prototree.WritableSubscriptionImpl, fetchHistoryOpts *prototree.FetchHistoryOpts) {
		require.Equal(t, expectedStateURI, stateURI)
		require.Equal(t, expectedKeypath, keypath)
		require.Equal(t, expectedSubType, subType)
		require.Equal(t, expectedWriteSubImpl, writeSubImpl)
		require.Equal(t, expectedFetchHistoryOpts, fetchHistoryOpts)
		callback1.ItHappened()
	})

	transport.OnWritableSubscriptionOpened(func(stateURI string, keypath state.Keypath, subType prototree.SubscriptionType, writeSubImpl prototree.WritableSubscriptionImpl, fetchHistoryOpts *prototree.FetchHistoryOpts) {
		require.Equal(t, expectedStateURI, stateURI)
		require.Equal(t, expectedKeypath, keypath)
		require.Equal(t, expectedSubType, subType)
		require.Equal(t, expectedWriteSubImpl, writeSubImpl)
		require.Equal(t, expectedFetchHistoryOpts, fetchHistoryOpts)
		callback2.ItHappened()
	})

	transport.HandleWritableSubscriptionOpened(expectedStateURI, expectedKeypath, expectedSubType, expectedWriteSubImpl, expectedFetchHistoryOpts)
	callback1.AwaitOrFail(t, 1*time.Second)
	callback2.AwaitOrFail(t, 1*time.Second)
}
