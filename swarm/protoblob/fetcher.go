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
	blobStore      blob.Store
	searchForPeers func(ctx context.Context, blobID blob.ID) <-chan BlobPeerConn
	peerPool       swarm.PeerPool[BlobPeerConn]
	workPool       *workPool
	// getPeerBackoff utils.ExponentialBackoff
}

func newFetcher(
	blobID blob.ID,
	blobStore blob.Store,
	searchForPeers func(ctx context.Context, blobID blob.ID) <-chan BlobPeerConn,
) *fetcher {
	return &fetcher{
		Process:        *process.New("fetcher " + blobID.String()),
		Logger:         log.NewLogger("protoblob"),
		blobID:         blobID,
		blobStore:      blobStore,
		searchForPeers: searchForPeers,
		peerPool:       nil,
		workPool:       nil,
		// getPeerBackoff: utils.ExponentialBackoff{Min: 1 * time.Second, Max: 10 * time.Second},
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
			f.Errorf("while verifying blob %v: %+v", f.blobID, err)
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

		err = f.startWorkPool(manifest.Chunks)
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

func (f *fetcher) startPeerPool() (err error) {
	defer errors.AddStack(&err)

	maxConns, err := f.blobStore.MaxFetchConns()
	if err != nil {
		return err
	}

	f.peerPool = swarm.NewPeerPool[BlobPeerConn](
		maxConns,
		func(ctx context.Context) (<-chan BlobPeerConn, error) {
			ctx, _ = context.WithTimeout(ctx, 10*time.Second)
			return f.searchForPeers(ctx, f.blobID), nil
		},
	)
	return f.Process.SpawnChild(nil, f.peerPool)
}

func (f *fetcher) fetchManifest(ctx context.Context) (_ blob.Manifest, err error) {
	defer errors.AddStack(&err)

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
			// time.Sleep(f.getPeerBackoff.Next())
			continue
		}
		// f.getPeerBackoff.Reset()

		manifest, err := blobPeer.FetchBlobManifest(ctx, f.blobID)
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

func (f *fetcher) startWorkPool(chunks []blob.ManifestChunk) (err error) {
	defer errors.AddStack(&err)

	var jobs []types.Hash
	for _, chunk := range chunks {
		have, err := f.blobStore.HaveChunk(chunk.SHA3)
		if err != nil {
			return errors.Wrap(err, "while reading from blob store")
		}
		if !have {
			jobs = append(jobs, chunk.SHA3)
		}
	}

	f.workPool = newWorkPool(jobs)
	return f.Process.SpawnChild(nil, f.workPool)
}

func (f *fetcher) fetchChunks(ctx context.Context) (err error) {
	defer errors.AddStack(&err)

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
			// time.Sleep(f.getPeerBackoff.Next())
			continue
		}
		// f.getPeerBackoff.Reset()

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

func (f *fetcher) getPeer(ctx context.Context) (_ BlobPeerConn, err error) {
	defer errors.AddStack(&err)

	for {
		peer, err := f.peerPool.GetPeer(ctx)
		if err != nil {
			return nil, err
		}
		if peer == nil || reflect.ValueOf(peer).IsNil() {
			panic("peer is nil")
		} else if !peer.Dialable() {
			f.peerPool.ReturnPeer(peer, true)
			continue
		} else if !peer.Ready() {
			f.peerPool.ReturnPeer(peer, false)
			continue
		}

		return peer, nil
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

		sha3, i, ok := f.workPool.NextJob()
		if !ok {
			return nil
		}

		chunk, err := peer.FetchBlobChunk(ctx, sha3)
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

type workPool struct {
	process.Process
	jobs           map[types.Hash]int
	chJobs         chan types.Hash
	chJobsComplete chan struct{}
}

func newWorkPool(jobs []types.Hash) *workPool {
	p := &workPool{
		Process:        *process.New("work pool"),
		jobs:           make(map[types.Hash]int),
		chJobs:         make(chan types.Hash, len(jobs)),
		chJobsComplete: make(chan struct{}),
	}
	for i, job := range jobs {
		p.jobs[job] = i
		p.chJobs <- job
	}
	return p
}

func (p *workPool) Start() error {
	err := p.Process.Start()
	if err != nil {
		return err
	}
	defer p.Process.Autoclose()

	p.Process.Go(nil, "await completion", func(ctx context.Context) {
		numJobs := len(p.jobs)
		for i := 0; i < numJobs; i++ {
			select {
			case <-p.chJobsComplete:
			case <-ctx.Done():
				return
			}
		}
	})
	return nil
}

func (p *workPool) NextJob() (job types.Hash, i int, ok bool) {
	select {
	case <-p.Ctx().Done():
		return types.Hash{}, 0, false
	case job = <-p.chJobs:
		return job, p.jobs[job], true
	}
}

func (p *workPool) ReturnFailedJob(job types.Hash) {
	select {
	case <-p.Ctx().Done():
	case p.chJobs <- job:
	}
}

func (p *workPool) MarkJobComplete() {
	select {
	case <-p.Ctx().Done():
	case p.chJobsComplete <- struct{}{}:
	}
}

func (p *workPool) NumJobs() int {
	return len(p.jobs)
}
