package protoblob

import (
	"context"
	"reflect"
	"time"

	"go.uber.org/multierr"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type fetcher struct {
	process.Process
	log.Logger
	blobID         blob.ID
	maxConns       uint64
	blobStore      blob.Store
	searchForPeers func(ctx context.Context, blobID blob.ID) <-chan BlobPeerConn
	peerPool       swarm.PeerPool
	workPool       *swarm.WorkPool
	getPeerBackoff utils.ExponentialBackoff
}

func newFetcher(
	blobID blob.ID,
	maxConns uint64,
	blobStore blob.Store,
	searchForPeers func(ctx context.Context, blobID blob.ID) <-chan BlobPeerConn,
) *fetcher {
	return &fetcher{
		Process:        *process.New("fetcher " + blobID.String()),
		Logger:         log.NewLogger("blob proto"),
		blobID:         blobID,
		maxConns:       maxConns,
		blobStore:      blobStore,
		searchForPeers: searchForPeers,
		peerPool:       nil,
		workPool:       nil,
		getPeerBackoff: utils.ExponentialBackoff{Min: 1 * time.Second, Max: 10 * time.Second},
	}
}

func (f *fetcher) Start() error {
	err := f.Process.Start()
	if err != nil {
		return err
	}
	defer f.Process.AutocloseWithCleanup(func() {
		err := f.blobStore.VerifyBlobOrPrune(f.blobID)
		if err != nil {
			f.Errorf("while verifying blob %v: %v", f.blobID, err)
		}
	})

	err = f.startPeerPool()
	if err != nil {
		return err
	}

	f.Process.Go(nil, "runloop", func(ctx context.Context) {
		defer f.peerPool.Close()

		manifest, err := f.fetchManifest(ctx)
		if err != nil {
			f.Errorf("while fetching manifest: %v", err)
			return
		}

		err = f.startWorkPool(manifest.ChunkSHA3s)
		if err != nil {
			f.Errorf("while starting work pool: %v", err)
			return
		}

		err = f.fetchChunks(ctx)
		if err != nil {
			return
		}
	})
	return nil
}

func (f *fetcher) Close() error {
	return multierr.Append(
		f.blobStore.VerifyBlobOrPrune(f.blobID),
		f.Process.Close(),
	)
}

func (f *fetcher) startPeerPool() error {
	restartSearchBackoff := utils.ExponentialBackoff{Min: 3 * time.Second, Max: 10 * time.Second}

	f.peerPool = swarm.NewPeerPool(
		f.maxConns,
		func(ctx context.Context) (<-chan swarm.PeerConn, error) {
			select {
			case <-ctx.Done():
				return nil, nil
			case <-f.peerPool.Done():
				return nil, nil
			case <-time.After(restartSearchBackoff.Next()):
			}
			chBlobPeers := f.searchForPeers(ctx, f.blobID)
			return f.convertBlobPeerChan(ctx, chBlobPeers), nil // Can't wait for generics
		},
	)
	return f.Process.SpawnChild(nil, f.peerPool)
}

func (f *fetcher) fetchManifest(ctx context.Context) (_ blob.Manifest, err error) {
	manifest, err := f.blobStore.Manifest(f.blobID)
	if err != nil && errors.Cause(err) != errors.Err404 {
		return blob.Manifest{}, err
	} else if err == nil {
		return manifest, nil
	}

	for {
		select {
		case <-ctx.Done():
			return blob.Manifest{}, ctx.Err()
		default:
		}

		blobPeer, err := f.getPeer(ctx)
		if err != nil {
			f.Errorf("error getting peer from pool: %v", err)
			time.Sleep(f.getPeerBackoff.Next())
			continue
		}
		f.getPeerBackoff.Reset()

		manifest, err := blobPeer.FetchBlobManifest(f.blobID)
		if err != nil {
			f.peerPool.ReturnPeer(blobPeer, false)
			f.Errorf("error getting peer from pool: %v", err)
			continue
		}
		f.peerPool.ReturnPeer(blobPeer, false)

		err = f.blobStore.StoreManifest(blob.ID{HashAlg: types.SHA3, Hash: f.blobID.Hash}, manifest)
		if err != nil {
			f.Errorf("error storing manifest: %v", err)
			return blob.Manifest{}, err
		}
		f.Debugf("fetched manifest for blob %v", f.blobID)
		return manifest, nil
	}
}

func (f *fetcher) startWorkPool(chunkSHA3s []types.Hash) error {
	var jobs []interface{}
	for _, chunkSHA3 := range chunkSHA3s {
		have, err := f.blobStore.HaveChunk(chunkSHA3)
		if err != nil {
			return errors.Wrap(err, "while reading from blob store")
		}
		if !have {
			jobs = append(jobs, chunkSHA3)
		}
	}

	f.workPool = swarm.NewWorkPool(jobs)
	return f.Process.SpawnChild(nil, f.workPool)
}

func (f *fetcher) fetchChunks(ctx context.Context) error {
	ctx, cancel := utils.CombinedContext(ctx, f.workPool.Done())
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-f.workPool.Done():
			return nil
		default:
		}

		blobPeer, err := f.getPeer(ctx)
		if errors.Cause(err) == context.Canceled {
			return err
		} else if err != nil {
			f.Errorf("error getting peer from pool: %v", err)
			time.Sleep(f.getPeerBackoff.Next())
			continue
		}
		f.getPeerBackoff.Reset()

		f.Process.Go(nil, "readUntilErrorOrShutdown "+blobPeer.DialInfo().String(), func(ctx context.Context) {
			defer f.peerPool.ReturnPeer(blobPeer, false)

			err := f.readUntilErrorOrShutdown(ctx, blobPeer)
			if err != nil {
				f.Errorf("while downloading blob chunks: %v (peer: %v, blobID: %v)", err, blobPeer.DialInfo(), f.blobID)
				return
			}
		})
	}
}

func (f *fetcher) getPeer(ctx context.Context) (BlobPeerConn, error) {
	for {
		peer, err := f.peerPool.GetPeer(ctx)
		if err != nil {
			return nil, err
		} else if peer == nil || reflect.ValueOf(peer).IsNil() {
			panic("peer is nil")
		} else if !peer.Dialable() {
			f.peerPool.ReturnPeer(peer, true)
			continue
		} else if !peer.Ready() {
			f.peerPool.ReturnPeer(peer, false)
			continue
		}

		// Ensure the peer supports the blob protocol
		blobPeer, is := peer.(BlobPeerConn)
		if !is {
			// If not, strike it so the pool doesn't return it again
			f.peerPool.ReturnPeer(peer, true)
			continue
		}
		return blobPeer, nil
	}
}

func (f *fetcher) readUntilErrorOrShutdown(ctx context.Context, peer BlobPeerConn) error {
	err := peer.EnsureConnected(ctx)
	if err != nil {
		return err
	}
	defer peer.Close()

	for {
		select {
		case <-ctx.Done():
			return errors.ErrClosed
		case <-f.workPool.Done():
			return nil
		default:
		}

		x, i, ok := f.workPool.NextJob()
		if !ok {
			return nil
		}
		sha3, ok := x.(types.Hash)
		if !ok {
			panic("invariant violation")
		}

		chunk, err := peer.FetchBlobChunk(sha3)
		if err != nil {
			f.workPool.ReturnFailedJob(sha3)
			return errors.Wrapf(err, "while fetching chunk %v", sha3)
		}

		err = f.blobStore.StoreChunkIfHashMatches(sha3, chunk)
		if err != nil {
			f.workPool.ReturnFailedJob(sha3)
			return errors.Wrapf(err, "while storing chunk %v", sha3)
		}
		f.Debugf("fetched chunk %v (%v/%v) for blob %v", sha3, i+1, f.workPool.NumJobs(), f.blobID)
		f.workPool.MarkJobComplete()

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func (f *fetcher) convertBlobPeerChan(ctx context.Context, ch <-chan BlobPeerConn) <-chan swarm.PeerConn {
	chPeer := make(chan swarm.PeerConn)
	go func() {
		defer close(chPeer)
		for {
			select {
			case <-f.Process.Done():
				return
			case <-ctx.Done():
				return
			case peer, open := <-ch:
				if !open {
					return
				}

				select {
				case <-f.Process.Done():
					return
				case <-ctx.Done():
					return
				case chPeer <- peer:
				}
			}
		}
	}()
	return chPeer
}
