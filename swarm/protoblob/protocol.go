package protoblob

import (
	"context"
	"time"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm"
	"redwood.dev/types"
	"redwood.dev/utils"
)

//go:generate mockery --name BlobProtocol --output ./mocks/ --case=underscore
type BlobProtocol interface {
	process.Interface
	ProvidersOfBlob(ctx context.Context, blobID blob.ID) <-chan BlobPeerConn
}

//go:generate mockery --name BlobTransport --output ./mocks/ --case=underscore
type BlobTransport interface {
	swarm.Transport
	ProvidersOfBlob(ctx context.Context, blobID blob.ID) (<-chan BlobPeerConn, error)
	AnnounceBlobs(ctx context.Context, blobIDs types.Set[blob.ID])
	OnBlobManifestRequest(handler func(blobID blob.ID, peer BlobPeerConn))
	OnBlobChunkRequest(handler func(sha3 types.Hash, peer BlobPeerConn))
}

//go:generate mockery --name BlobPeerConn --output ./mocks/ --case=underscore
type BlobPeerConn interface {
	swarm.PeerConn
	FetchBlobManifest(blobID blob.ID) (blob.Manifest, error)
	SendBlobManifest(m blob.Manifest, exists bool) error
	FetchBlobChunk(sha3 types.Hash) ([]byte, error)
	SendBlobChunk(chunk []byte, exists bool) error
}

type blobProtocol struct {
	swarm.BaseProtocol[BlobTransport, BlobPeerConn]

	blobStore         blob.Store
	transports        map[string]BlobTransport
	blobsNeeded       *utils.Mailbox[[]blob.ID]
	blobsBeingFetched types.SyncSet[blob.ID]

	// fetchBlobsTask    *fetchBlobsTask
	announceBlobsTask *announceBlobsTask
}

const (
	ProtocolName = "protoblob"
)

func NewBlobProtocol(transports []swarm.Transport, blobStore blob.Store) *blobProtocol {
	transportsMap := make(map[string]BlobTransport)
	for _, tpt := range transports {
		if tpt, is := tpt.(BlobTransport); is {
			transportsMap[tpt.Name()] = tpt
		}
	}
	bp := &blobProtocol{
		BaseProtocol: swarm.BaseProtocol[BlobTransport, BlobPeerConn]{
			Process:    *process.New(ProtocolName),
			Logger:     log.NewLogger(ProtocolName),
			Transports: transportsMap,
		},
		blobStore:         blobStore,
		transports:        transportsMap,
		blobsNeeded:       utils.NewMailbox[[]blob.ID](0),
		blobsBeingFetched: types.NewSyncSet[blob.ID](nil),
	}

	// bp.fetchBlobsTask = NewFetchBlobsTask(15*time.Second, bp)
	bp.announceBlobsTask = NewAnnounceBlobsTask(15*time.Second, bp)

	bp.blobStore.OnBlobsNeeded(func(blobs []blob.ID) {
		bp.blobsNeeded.Deliver(blobs)
		// bp.fetchBlobsTask.Enqueue()
	})
	bp.blobStore.OnBlobsSaved(func() {
		bp.announceBlobsTask.Enqueue()
	})

	for _, tpt := range bp.transports {
		bp.Infof(0, "registering %v", tpt.Name())
		tpt.OnBlobManifestRequest(bp.handleBlobManifestRequest)
		tpt.OnBlobChunkRequest(bp.handleBlobChunkRequest)
	}

	return bp
}

func (bp *blobProtocol) Name() string {
	return ProtocolName
}

func (bp *blobProtocol) Start() error {
	err := bp.Process.Start()
	if err != nil {
		return err
	}

	bp.periodicallyFetchMissingBlobs()

	// err = bp.Process.SpawnChild(nil, bp.fetchBlobsTask)
	// if err != nil {
	// 	return err
	// }

	err = bp.Process.SpawnChild(nil, bp.announceBlobsTask)
	if err != nil {
		return err
	}
	bp.announceBlobsTask.Enqueue()

	return nil
}

func (bp *blobProtocol) Close() error {
	bp.Infof(0, "blob protocol shutting down")
	return bp.Process.Close()
}

func (bp *blobProtocol) ProvidersOfBlob(ctx context.Context, blobID blob.ID) <-chan BlobPeerConn {
	ch := make(chan BlobPeerConn)

	child := bp.Process.NewChild(ctx, "ProvidersOfBlob "+blobID.String())
	defer child.AutocloseWithCleanup(func() {
		close(ch)
	})

	for _, tpt := range bp.transports {
		innerCh, err := tpt.ProvidersOfBlob(ctx, blobID)
		if err != nil {
			bp.Warnf("transport %v could not fetch providers of blob %v", tpt.Name(), blobID)
			continue
		}

		child.Go(ctx, tpt.Name(), func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					return
				case peer, open := <-innerCh:
					if !open {
						return
					}

					select {
					case <-ctx.Done():
						return
					case ch <- peer:
					}
				}
			}
		})
	}
	return ch
}

func (bp *blobProtocol) periodicallyFetchMissingBlobs() {
	bp.Process.Go(nil, "periodicallyFetchMissingBlobs", func(ctx context.Context) {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-bp.blobsNeeded.Notify():
				blobsBlobs := bp.blobsNeeded.RetrieveAll()
				var allBlobs []blob.ID
				for _, blobs := range blobsBlobs {
					allBlobs = append(allBlobs, blobs...)
				}
				bp.fetchBlobs(allBlobs)

			case <-ticker.C:
				blobs, err := bp.blobStore.BlobsNeeded()
				if err != nil {
					bp.Errorf("error fetching list of needed blobs: %v", err)
					continue
				}
				if len(blobs) > 0 {
					bp.fetchBlobs(blobs)
				}
			}
		}
	})
}

// @@TODO: maybe limit the number of blob fetchers that are active at any given time
func (bp *blobProtocol) fetchBlobs(blobs []blob.ID) {
	for _, blobID := range blobs {
		if !bp.claimBlobForFetcher(blobID) {
			bp.Errorf("could not claim blob %v for fetcher", blobID)
			continue
		}

		fetcher := newFetcher(blobID, bp.blobStore, bp.ProvidersOfBlob)

		err := bp.Process.SpawnChild(nil, fetcher)
		if err != nil {
			bp.Errorf("error spawning blob fetcher (blobID: %v): %v", blobID, err)
			return
		}

		blobID := blobID
		go func() {
			defer bp.unclaimBlobForFetcher(blobID)
			<-fetcher.Done()
			bp.Successf("fetcher finished (%v)", blobID)
		}()
	}
}

func (bp *blobProtocol) claimBlobForFetcher(blobID blob.ID) (ok bool) {
	claimed := bp.blobsBeingFetched.Add(blobID)
	return !claimed
}

func (bp *blobProtocol) unclaimBlobForFetcher(blobID blob.ID) {
	bp.blobsBeingFetched.Remove(blobID)
}

func (bp *blobProtocol) handleBlobManifestRequest(blobID blob.ID, peer BlobPeerConn) {
	defer peer.Close()

	bp.Debugf("incoming blob manifest request for %v from %v", blobID, peer.DialInfo())

	manifest, err := bp.blobStore.Manifest(blobID)
	if errors.Cause(err) == errors.Err404 {
		err := peer.SendBlobManifest(blob.Manifest{}, false)
		if err != nil {
			bp.Errorf("while responding to manifest request (blobID: %v, peer: %v): %v", blobID, peer.DialInfo(), err)
			return
		}
	} else if err != nil {
		bp.Errorf("while querying blob store for manifest (blobID: %v): %v", blobID, err)
		return
	}

	err = peer.SendBlobManifest(manifest, true)
	if err != nil {
		bp.Errorf("while responding to manifest request (blobID: %v, peer: %v): %v", blobID, peer.DialInfo(), err)
		return
	}
}

// @@TODO: refactor this to not close the stream and to respond to as many
// chunk requests as needed, might be a minor performance improvement
func (bp *blobProtocol) handleBlobChunkRequest(sha3 types.Hash, peer BlobPeerConn) {
	defer peer.Close()

	bp.Debugf("incoming blob chunk request for %v from %v", sha3.Hex(), peer.DialInfo())

	chunkBytes, err := bp.blobStore.Chunk(sha3)
	if errors.Cause(err) == errors.Err404 {
		err := peer.SendBlobChunk(nil, false)
		if err != nil {
			bp.Errorf("while sending blob chunk response: %v", err)
			return
		}

	} else if err != nil {
		bp.Errorf("while fetching blob chunk: %v", err)
		return
	}

	err = peer.SendBlobChunk(chunkBytes, true)
	if err != nil {
		bp.Errorf("while sending blob chunk response: %v", err)
		return
	}
}

type announceBlobsTask struct {
	process.PeriodicTask
	log.Logger
	blobProto *blobProtocol
	interval  time.Duration
}

func NewAnnounceBlobsTask(
	interval time.Duration,
	blobProto *blobProtocol,
) *announceBlobsTask {
	t := &announceBlobsTask{
		Logger:    log.NewLogger(ProtocolName),
		blobProto: blobProto,
		interval:  interval,
	}
	t.PeriodicTask = *process.NewPeriodicTask("AnnounceBlobsTask", utils.NewStaticTicker(interval), t.announceBlobs)
	return t
}

func (t *announceBlobsTask) announceBlobs(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, t.interval)

	child := t.Process.NewChild(ctx, "announceBlobs")
	defer child.AutocloseWithCleanup(cancel)

	sha1s, sha3s, err := t.blobProto.blobStore.BlobIDs()
	if err != nil {
		t.Errorf("while fetching blob IDs from blob store: %v", err)
		return
	}

	blobIDs := types.NewSet[blob.ID](append(sha1s, sha3s...))

	for _, tpt := range t.blobProto.Transports {
		tpt.AnnounceBlobs(ctx, blobIDs)
	}
}
