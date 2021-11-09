package blob_test

import (
	"bytes"
	"crypto/sha1"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/internal/testutils"
	"redwood.dev/types"
)

func TestStore(t *testing.T) {
	foo := []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Duis magna odio, malesuada sed tortor ut, mollis hendrerit enim. Etiam et nulla lorem. Fusce sollicitudin tortor neque, sed iaculis diam facilisis in. Vivamus pellentesque dapibus magna quis ultricies. Praesent at erat dignissim, pellentesque mauris euismod, condimentum mi. Praesent imperdiet felis dui, a feugiat mi efficitur ut. Vestibulum at semper turpis. Proin in felis sit amet ex rhoncus dapibus sit amet non odio. Sed purus magna, placerat et tortor sed, hendrerit fermentum mi. Etiam auctor vitae lorem ut egestas. Nulla quis justo tristique, facilisis justo sed, malesuada leo. Mauris accumsan bibendum lorem, vitae imperdiet odio semper sit amet. Sed gravida, tellus sit amet scelerisque molestie, leo erat aliquam velit, a finibus lacus leo sit amet tortor.")
	bar := []byte("Integer ac aliquam enim, ut tempus purus. Praesent euismod tempor lorem in pellentesque. Etiam et est sit amet orci mollis scelerisque. Etiam fringilla dictum nulla. Mauris dignissim rhoncus metus ut porttitor. Integer feugiat posuere odio, non tincidunt purus efficitur nec. Duis aliquet volutpat nulla, in ultrices est vehicula ac. Duis fringilla vitae neque a pellentesque. Proin pellentesque dictum tristique.")
	baz := []byte("Vivamus at finibus urna. Aliquam viverra faucibus dolor in pharetra. Sed at varius turpis, eget mattis neque. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Nunc at risus orci. Nam sodales posuere suscipit. Cras in egestas nisl. Nullam ornare orci at neque ullamcorper, non dapibus eros consequat. Quisque eu hendrerit nibh, eget tincidunt quam. Aliquam faucibus tortor eget elit aliquam rutrum. Donec leo nisl, ullamcorper at enim vel, porta ornare elit. Praesent vehicula at elit a gravida.")
	quux := []byte("Donec ultricies sagittis nulla, at posuere justo bibendum ut. Phasellus sit amet tempus nulla. Vivamus eget ex arcu. Maecenas bibendum tortor sed nibh tempus feugiat. Donec ullamcorper mollis arcu non vestibulum. Curabitur porttitor, odio quis lacinia cursus, augue enim vehicula tellus, id consectetur magna dui ut risus. Suspendisse molestie, lacus id ultrices varius, nunc mauris accumsan erat, ornare bibendum nibh nisl eu lectus. Suspendisse nec tellus vitae arcu sollicitudin facilisis congue eu turpis. In tristique erat elit, faucibus pellentesque libero sagittis eget. Aliquam eget nunc erat. Etiam in euismod mi. Nunc vel purus imperdiet, viverra lectus vel, sollicitudin justo.")
	zork := []byte("Phasellus convallis magna in fringilla laoreet. Aliquam ac orci non enim finibus suscipit non eget odio. Morbi finibus ante ut scelerisque maximus. Fusce consectetur id enim ac scelerisque. Nullam vulputate nisi ac est commodo, euismod condimentum ligula rhoncus. Donec eu magna nulla. Pellentesque in finibus est.")

	t.Run("will store a chunk with the correct hash", func(t *testing.T) {
		t.Parallel()

		store := blob.NewBadgerStore(t.TempDir(), nil)
		err := store.Start()
		require.NoError(t, err)
		defer store.Close()

		sha3 := types.HashBytes(foo)

		have, err := store.HaveChunk(sha3)
		require.NoError(t, err)
		require.False(t, have)

		err = store.StoreChunkIfHashMatches(sha3, foo)
		require.NoError(t, err)

		have, err = store.HaveChunk(sha3)
		require.NoError(t, err)
		require.True(t, have)

		bs, err := store.Chunk(sha3)
		require.NoError(t, err)
		require.Equal(t, foo, bs)
	})

	t.Run("will not store a chunk with the hash doesn't match", func(t *testing.T) {
		t.Parallel()

		store := blob.NewBadgerStore(t.TempDir(), nil)
		err := store.Start()
		require.NoError(t, err)
		defer store.Close()

		badSHA3 := types.Hash{0x1}
		goodSHA3 := types.HashBytes(foo)

		err = store.StoreChunkIfHashMatches(badSHA3, foo)
		require.Equal(t, blob.ErrWrongHash, errors.Cause(err))

		have, err := store.HaveChunk(badSHA3)
		require.NoError(t, err)
		require.False(t, have)

		have, err = store.HaveChunk(goodSHA3)
		require.NoError(t, err)
		require.False(t, have)
	})

	t.Run("will store a manifest", func(t *testing.T) {
		t.Parallel()

		store := blob.NewBadgerStore(t.TempDir(), nil)
		err := store.Start()
		require.NoError(t, err)
		defer store.Close()

		expectedManifest := blob.Manifest{
			Size: 12345,
			ChunkSHA3s: []types.Hash{
				testutils.RandomHash(t),
				testutils.RandomHash(t),
				testutils.RandomHash(t),
				testutils.RandomHash(t),
				testutils.RandomHash(t),
			},
		}

		blobID := blob.ID{HashAlg: types.SHA3, Hash: types.Hash{0x1}}

		have, err := store.HaveManifest(blobID)
		require.NoError(t, err)
		require.False(t, have)

		err = store.StoreManifest(blobID, expectedManifest)
		require.NoError(t, err)

		have, err = store.HaveManifest(blobID)
		require.NoError(t, err)
		require.True(t, have)

		manifest, err := store.Manifest(blobID)
		require.NoError(t, err)
		require.Equal(t, expectedManifest, manifest)
	})

	t.Run("marks blobs as needed", func(t *testing.T) {
		t.Parallel()

		store := blob.NewBadgerStore(t.TempDir(), nil)
		err := store.Start()
		require.NoError(t, err)
		defer store.Close()

		blobIDs, err := store.BlobsNeeded()
		require.NoError(t, err)
		require.Len(t, blobIDs, 0)

		expectedBlobIDs := []blob.ID{
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
		}
		err = store.MarkBlobsAsNeeded(expectedBlobIDs)
		require.NoError(t, err)

		blobIDs, err = store.BlobsNeeded()
		require.NoError(t, err)
		require.Len(t, blobIDs, len(expectedBlobIDs))
		for _, id := range expectedBlobIDs {
			require.Contains(t, blobIDs, id)
		}
	})

	t.Run("will not mark a blob as needed if it is present", func(t *testing.T) {
		t.Parallel()

		store := blob.NewBadgerStore(t.TempDir(), nil)
		err := store.Start()
		require.NoError(t, err)
		defer store.Close()

		blobIDs, err := store.BlobsNeeded()
		require.NoError(t, err)
		require.Len(t, blobIDs, 0)

		_, sha3, err := store.StoreBlob(io.NopCloser(bytes.NewReader(foo)))
		require.NoError(t, err)

		blobIDs = []blob.ID{
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
			{HashAlg: types.SHA3, Hash: sha3},
		}

		err = store.MarkBlobsAsNeeded(blobIDs)
		require.NoError(t, err)

		blobIDs, err = store.BlobsNeeded()
		require.NoError(t, err)

		expectedBlobIDs := blobIDs[:2]
		require.Len(t, blobIDs, len(expectedBlobIDs))
		for _, id := range expectedBlobIDs {
			require.Contains(t, blobIDs, id)
		}
	})

	t.Run("notifies subscribers of missing blobs when they are marked", func(t *testing.T) {
		t.Parallel()

		store := blob.NewBadgerStore(t.TempDir(), nil)
		err := store.Start()
		require.NoError(t, err)
		defer store.Close()

		expectedBlobIDs := []blob.ID{
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
		}

		callback1 := testutils.NewAwaiter()
		store.OnBlobsNeeded(func(blobIDs []blob.ID) {
			require.Len(t, blobIDs, len(expectedBlobIDs))
			for _, id := range expectedBlobIDs {
				require.Contains(t, blobIDs, id)
			}
			callback1.ItHappened()
		})

		callback2 := testutils.NewAwaiter()
		store.OnBlobsNeeded(func(blobIDs []blob.ID) {
			require.Len(t, blobIDs, len(expectedBlobIDs))
			for _, id := range expectedBlobIDs {
				require.Contains(t, blobIDs, id)
			}
			callback2.ItHappened()
		})

		err = store.MarkBlobsAsNeeded(expectedBlobIDs)
		require.NoError(t, err)

		callback1.AwaitOrFail(t)
		callback2.AwaitOrFail(t)

		// Ensure it happens again
		expectedBlobIDs = []blob.ID{
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
			{HashAlg: types.SHA3, Hash: testutils.RandomHash(t)},
		}

		err = store.MarkBlobsAsNeeded(expectedBlobIDs)
		require.NoError(t, err)

		callback1.AwaitOrFail(t)
		callback2.AwaitOrFail(t)
	})

	t.Run("does not notify subscribers of missing blobs if none are missing", func(t *testing.T) {
		t.Parallel()

		store := blob.NewBadgerStore(t.TempDir(), nil)
		err := store.Start()
		require.NoError(t, err)
		defer store.Close()

		blobIDs, err := store.BlobsNeeded()
		require.NoError(t, err)
		require.Len(t, blobIDs, 0)

		_, sha3, err := store.StoreBlob(io.NopCloser(bytes.NewReader(foo)))
		require.NoError(t, err)

		callback1 := testutils.NewAwaiter()
		callback2 := testutils.NewAwaiter()
		store.OnBlobsNeeded(func(blobIDs []blob.ID) { callback1.ItHappened() })
		store.OnBlobsNeeded(func(blobIDs []blob.ID) { callback2.ItHappened() })

		err = store.MarkBlobsAsNeeded([]blob.ID{{HashAlg: types.SHA3, Hash: sha3}})
		require.NoError(t, err)

		blobIDs, err = store.BlobsNeeded()
		require.NoError(t, err)
		require.Len(t, blobIDs, 0)

		callback1.NeverHappenedOrFail(t, 3*time.Second)
		callback2.NeverHappenedOrFail(t, 1*time.Second)
	})

	t.Run("notifies subscribers when blobs are saved via StoreBlob", func(t *testing.T) {
		t.Parallel()

		store := blob.NewBadgerStore(t.TempDir(), nil)
		err := store.Start()
		require.NoError(t, err)
		defer store.Close()

		callback1 := testutils.NewAwaiter()
		callback2 := testutils.NewAwaiter()
		store.OnBlobsSaved(func() { callback1.ItHappened() })
		store.OnBlobsSaved(func() { callback2.ItHappened() })

		_, _, err = store.StoreBlob(io.NopCloser(bytes.NewReader(foo)))
		require.NoError(t, err)

		callback1.AwaitOrFail(t)
		callback2.AwaitOrFail(t)

		// Ensure it happens again
		_, _, err = store.StoreBlob(io.NopCloser(bytes.NewReader(bar)))
		require.NoError(t, err)

		callback1.AwaitOrFail(t)
		callback2.AwaitOrFail(t)
	})

	t.Run("notifies subscribers when blobs are saved in chunks", func(t *testing.T) {
		t.Parallel()

		store := blob.NewBadgerStore(t.TempDir(), nil)
		err := store.Start()
		require.NoError(t, err)
		defer store.Close()

		callback1 := testutils.NewAwaiter()
		callback2 := testutils.NewAwaiter()
		store.OnBlobsSaved(func() { callback1.ItHappened() })
		store.OnBlobsSaved(func() { callback2.ItHappened() })

		fooHash := types.HashBytes(foo)
		barHash := types.HashBytes(bar)
		manifest := blob.Manifest{
			Size:       uint64(len(foo) + len(bar)),
			ChunkSHA3s: []types.Hash{fooHash, barHash},
		}
		blobID := blob.ID{
			HashAlg: types.SHA3,
			Hash:    types.HashBytes(append(foo, bar...)),
		}
		err = store.StoreManifest(blobID, manifest)
		require.NoError(t, err)

		err = store.StoreChunkIfHashMatches(fooHash, foo)
		require.NoError(t, err)

		err = store.StoreChunkIfHashMatches(barHash, bar)
		require.NoError(t, err)

		err = store.VerifyBlobOrPrune(blobID)
		require.NoError(t, err)

		callback1.AwaitOrFail(t)
		callback2.AwaitOrFail(t)
	})

	t.Run("AllHashes returns the IDs of all complete blobs", func(t *testing.T) {
		t.Parallel()

		sha1 := func(bs []byte) types.Hash {
			sha1Hasher := sha1.New()
			sha1Hasher.Write(bs)
			h, err := types.HashFromBytes(sha1Hasher.Sum(nil))
			require.NoError(t, err)
			return h
		}

		store := blob.NewBadgerStore(t.TempDir(), nil)
		err := store.Start()
		require.NoError(t, err)
		defer store.Close()

		// Store one using StoreBlob
		blob1SHA1, blob1SHA3, err := store.StoreBlob(io.NopCloser(bytes.NewReader(foo)))
		require.NoError(t, err)

		// Store one in chunks
		barSHA3 := types.HashBytes(bar)
		bazSHA3 := types.HashBytes(baz)
		blob2SHA3 := types.HashBytes(append(bar, baz...))
		blob2SHA1 := sha1(append(bar, baz...))
		manifest2 := blob.Manifest{
			Size:       uint64(len(bar) + len(baz)),
			ChunkSHA3s: []types.Hash{barSHA3, bazSHA3},
		}
		blobID2 := blob.ID{HashAlg: types.SHA3, Hash: blob2SHA3}
		err = store.StoreManifest(blobID2, manifest2)
		require.NoError(t, err)

		err = store.StoreChunkIfHashMatches(barSHA3, bar)
		require.NoError(t, err)

		err = store.StoreChunkIfHashMatches(bazSHA3, baz)
		require.NoError(t, err)

		err = store.VerifyBlobOrPrune(blobID2)
		require.NoError(t, err)

		// Store only one chunk of another one
		quuxSHA3 := types.HashBytes(quux)
		zorkSHA3 := types.HashBytes(zork)
		blob3SHA3 := types.HashBytes(append(quux, zork...))
		blob3SHA1 := sha1(append(quux, zork...))
		manifest3 := blob.Manifest{
			Size:       uint64(len(quux) + len(zork)),
			ChunkSHA3s: []types.Hash{quuxSHA3, zorkSHA3},
		}
		blobID3 := blob.ID{HashAlg: types.SHA3, Hash: blob3SHA3}
		err = store.StoreManifest(blobID3, manifest3)
		require.NoError(t, err)

		err = store.StoreChunkIfHashMatches(quuxSHA3, quux)
		require.NoError(t, err)

		have, err := store.HaveBlob(blob.ID{types.SHA1, blob1SHA1})
		require.NoError(t, err)
		require.True(t, have)
		have, err = store.HaveBlob(blob.ID{types.SHA3, blob1SHA3})
		require.NoError(t, err)
		require.True(t, have)

		have, err = store.HaveBlob(blob.ID{types.SHA1, blob2SHA1})
		require.NoError(t, err)
		require.True(t, have)
		have, err = store.HaveBlob(blob.ID{types.SHA3, blob2SHA3})
		require.NoError(t, err)
		require.True(t, have)

		have, err = store.HaveBlob(blob.ID{types.SHA1, blob3SHA1})
		require.NoError(t, err)
		require.False(t, have)
		have, err = store.HaveBlob(blob.ID{types.SHA3, blob3SHA3})
		require.NoError(t, err)
		require.False(t, have)

		iter := store.BlobIDs()
		defer iter.Close()

		var ids []blob.ID
		for iter.Rewind(); iter.Valid(); iter.Next() {
			if iter.Err() != nil {
				break
			}
			sha1, sha3 := iter.Current()
			ids = append(ids, sha1)
			ids = append(ids, sha3)
		}
		require.NoError(t, iter.Err())

		require.NoError(t, err)
		require.Len(t, ids, 4)
		require.Contains(t, ids, blob.ID{types.SHA3, blob1SHA3})
		require.Contains(t, ids, blob.ID{types.SHA1, blob1SHA1})
		require.Contains(t, ids, blob.ID{types.SHA3, blob2SHA3})
		require.Contains(t, ids, blob.ID{types.SHA1, blob2SHA1})

		err = store.VerifyBlobOrPrune(blobID3)
		require.Equal(t, errors.Err404, errors.Cause(err))

		require.Len(t, ids, 4)
		require.Contains(t, ids, blob.ID{types.SHA3, blob1SHA3})
		require.Contains(t, ids, blob.ID{types.SHA1, blob1SHA1})
		require.Contains(t, ids, blob.ID{types.SHA3, blob2SHA3})
		require.Contains(t, ids, blob.ID{types.SHA1, blob2SHA1})
	})
}
