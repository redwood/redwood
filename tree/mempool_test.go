package tree_test

import (
	"sync/atomic"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"

	"redwood.dev/tree"
	"redwood.dev/types"
)

func TestMempool(t *testing.T) {
	g := NewGomegaWithT(t)

	t.Run("it does not re-process successful transactions", func(t *testing.T) {
		var count uint32
		mempool := tree.NewMempool(func(tx *tree.Tx) tree.ProcessTxOutcome {
			atomic.AddUint32(&count, 1)
			return tree.ProcessTxOutcome_Succeeded
		})

		err := mempool.Start()
		require.NoError(t, err)
		defer mempool.Close()

		mempool.Add(&tree.Tx{ID: types.RandomID()})
		mempool.Add(&tree.Tx{ID: types.RandomID()})

		g.Eventually(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(2)))
		g.Consistently(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(2)))

		mempool.Add(&tree.Tx{ID: types.RandomID()})

		g.Eventually(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(3)))
		g.Consistently(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(3)))
	})

	t.Run("does not re-process pending transactions if none of the current batch succeeded", func(t *testing.T) {
		var count uint32
		mempool := tree.NewMempool(func(tx *tree.Tx) tree.ProcessTxOutcome {
			atomic.AddUint32(&count, 1)
			return tree.ProcessTxOutcome_Retry
		})

		err := mempool.Start()
		require.NoError(t, err)
		defer mempool.Close()

		mempool.Add(&tree.Tx{ID: types.RandomID()})
		mempool.Add(&tree.Tx{ID: types.RandomID()})
		mempool.Add(&tree.Tx{ID: types.RandomID()})

		g.Eventually(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(3)))
		g.Consistently(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(3)))
	})

	t.Run("re-processes pending transactions if some of the current batch succeeded", func(t *testing.T) {
		var count uint32
		mempool := tree.NewMempool(func(tx *tree.Tx) tree.ProcessTxOutcome {
			if atomic.AddUint32(&count, 1) == 2 {
				return tree.ProcessTxOutcome_Succeeded
			}
			return tree.ProcessTxOutcome_Retry
		})

		err := mempool.Start()
		require.NoError(t, err)
		defer mempool.Close()

		mempool.Add(&tree.Tx{ID: types.RandomID()})
		mempool.Add(&tree.Tx{ID: types.RandomID()})
		mempool.Add(&tree.Tx{ID: types.RandomID()})

		g.Eventually(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(5)))
		g.Consistently(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(5)))
	})

	t.Run("never re-processes failed transactions", func(t *testing.T) {
		var count uint32
		mempool := tree.NewMempool(func(tx *tree.Tx) tree.ProcessTxOutcome {
			if atomic.AddUint32(&count, 1) == 1 {
				return tree.ProcessTxOutcome_Failed
			}
			return tree.ProcessTxOutcome_Succeeded
		})

		err := mempool.Start()
		require.NoError(t, err)
		defer mempool.Close()

		mempool.Add(&tree.Tx{ID: types.RandomID()})
		mempool.Add(&tree.Tx{ID: types.RandomID()})

		g.Eventually(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(2)))
		g.Consistently(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(2)))

		mempool.Add(&tree.Tx{ID: types.RandomID()})

		g.Eventually(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(3)))
		g.Consistently(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(3)))
	})

	t.Run("ignores duplicate transactions", func(t *testing.T) {
		var count uint32
		mempool := tree.NewMempool(func(tx *tree.Tx) tree.ProcessTxOutcome {
			atomic.AddUint32(&count, 1)
			return tree.ProcessTxOutcome_Succeeded
		})

		err := mempool.Start()
		require.NoError(t, err)
		defer mempool.Close()

		id := types.RandomID()

		mempool.Add(&tree.Tx{ID: id})
		mempool.Add(&tree.Tx{ID: id})

		g.Eventually(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(1)))
		g.Consistently(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(1)))
	})

	t.Run("does not process transactions after .Close() is called", func(t *testing.T) {
		var count uint32
		mempool := tree.NewMempool(func(tx *tree.Tx) tree.ProcessTxOutcome {
			atomic.AddUint32(&count, 1)
			return tree.ProcessTxOutcome_Succeeded
		})

		err := mempool.Start()
		require.NoError(t, err)

		mempool.Add(&tree.Tx{ID: types.RandomID()})

		g.Eventually(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(1)))
		g.Consistently(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(1)))

		mempool.Close()
		mempool.Add(&tree.Tx{ID: types.RandomID()})

		g.Eventually(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(1)))
		g.Consistently(func() uint32 { return atomic.LoadUint32(&count) }).Should(Equal(uint32(1)))
	})
}
