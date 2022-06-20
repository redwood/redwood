package blob_test

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"

	"redwood.dev/blob"
	"redwood.dev/types"
	"redwood.dev/utils"
	"redwood.dev/utils/badgerutils"
)

func TestReader_NoRange(t *testing.T) {
	t.Parallel()

	var badgerOpts badgerutils.OptsBuilder
	store := blob.NewBadgerStore(badgerOpts.ForPath(t.TempDir()))
	err := store.Start()
	require.NoError(t, err)
	defer store.Close()

	content := utils.RandomBytes(int(10 * utils.MB))

	_, _, err = store.StoreBlob(io.NopCloser(bytes.NewReader(content)))
	require.NoError(t, err)

	manifest, err := store.Manifest(blob.ID{HashAlg: types.SHA3, Hash: types.HashBytes(content)})
	require.NoError(t, err)

	err = iotest.TestReader(blob.NewReader(store.HelperGetDB(), manifest, nil), content)
	require.NoError(t, err)
}

func TestReader_Range(t *testing.T) {
	t.Parallel()

	var badgerOpts badgerutils.OptsBuilder
	store := blob.NewBadgerStore(badgerOpts.ForPath(t.TempDir()))
	err := store.Start()
	require.NoError(t, err)
	defer store.Close()

	for i := 0; i < 10; i++ {
		t.Run("", func(t *testing.T) {
			content := []byte(utils.RandomString(int(100 * utils.MB)))

			start := uint64(rand.Intn(len(content)))
			end := uint64(rand.Intn(len(content)))
			reverse := start > end
			rng := &types.Range{start, end, reverse}

			_, _, err = store.StoreBlob(io.NopCloser(bytes.NewReader(content)))
			require.NoError(t, err)

			manifest, err := store.Manifest(blob.ID{HashAlg: types.SHA3, Hash: types.HashBytes(content)})
			require.NoError(t, err)

			sliceRange := rng.NormalizedForLength(uint64(len(content)))
			err = iotest.TestReader(blob.NewReader(store.HelperGetDB(), manifest, rng), content[sliceRange.Start:sliceRange.End])
			require.NoError(t, err)
		})
	}
}
