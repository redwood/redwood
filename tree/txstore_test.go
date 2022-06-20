package tree_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/state"
	"redwood.dev/tree"
	"redwood.dev/tree/mocks"
)

func TestAllTxsForStateURIIterator(t *testing.T) {
	t.Parallel()

	stateURI := "foo.bar/blah"
	txStore := new(mocks.TxStore)

	txs := []tree.Tx{
		{StateURI: stateURI, ID: state.RandomVersion()},
		{StateURI: stateURI, ID: state.RandomVersion()},
		{StateURI: stateURI, ID: state.RandomVersion()},
		{StateURI: stateURI, ID: state.RandomVersion()},
		{StateURI: stateURI, ID: state.RandomVersion()},
	}

	var txIDs []state.Version
	for _, tx := range txs {
		txIDs = append(txIDs, tx.ID)
		txStore.On("FetchTx", stateURI, tx.ID).Return(tx, nil).Once()
	}

	iter := tree.NewAllTxsIterator(txStore, stateURI, txIDs)

	var i int
	for iter.Rewind(); iter.Valid(); iter.Next() {
		require.Equal(t, txs[i], *iter.Tx())
		i++
	}

	txStore.AssertExpectations(t)
}

func TestAllValidTxsForStateURIOrderedIterator(t *testing.T) {
	t.Parallel()

	stateURI := "foo.bar/blah"
	txStore := new(mocks.TxStore)

	txs := []tree.Tx{
		{StateURI: stateURI, ID: state.Version{0x1}, Status: tree.TxStatusValid},
		{StateURI: stateURI, ID: state.Version{0x2}, Status: tree.TxStatusValid},
		{StateURI: stateURI, ID: state.Version{0x3}, Status: tree.TxStatusValid},
		{StateURI: stateURI, ID: state.Version{0x4}, Status: tree.TxStatusValid},
		{StateURI: stateURI, ID: state.Version{0x5}, Status: tree.TxStatusValid},
		{StateURI: stateURI, ID: state.Version{0x6}, Status: tree.TxStatusValid},
	}
	txs[0].Children = []state.Version{txs[1].ID, txs[2].ID}
	txs[1].Children = []state.Version{txs[3].ID}
	txs[2].Children = []state.Version{txs[4].ID}
	txs[3].Children = []state.Version{txs[5].ID}
	txs[4].Children = []state.Version{txs[5].ID}

	txs[1].Parents = []state.Version{txs[0].ID}
	txs[2].Parents = []state.Version{txs[0].ID}
	txs[3].Parents = []state.Version{txs[1].ID}
	txs[4].Parents = []state.Version{txs[2].ID}
	txs[5].Parents = []state.Version{txs[3].ID, txs[4].ID}

	var txIDs []state.Version
	for _, tx := range txs {
		txIDs = append(txIDs, tx.ID)
		txStore.On("FetchTx", stateURI, tx.ID).Return(tx, nil).Once()
	}

	iter := tree.NewAllValidTxsForStateURIOrderedIterator(txStore, stateURI, txs[0].ID)

	var i int
	for iter.Rewind(); iter.Valid(); iter.Next() {
		require.Equal(t, txs[i], *iter.Tx())
		i++
	}
	require.NoError(t, iter.Err())

	txStore.AssertExpectations(t)
}
