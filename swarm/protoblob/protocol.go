package protoblob

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/blob"
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
	AnnounceBlob(ctx context.Context, blobID blob.ID) error
	OnBlobManifestRequest(handler func(blobID blob.ID, peer BlobPeerConn))
	OnBlobChunkRequest(handler func(sha3 types.Hash, peer BlobPeerConn))
}

//go:generate mockery --name BlobPeerConn --output ./mocks/ --case=underscore
type BlobPeerConn interface {
	swarm.PeerConn
	FetchBlobManifest(blobID blob.ID) (blob.Manifest, error)
	ReadBlobManifestRequest() (blob.ID, error)
	SendBlobManifest(m blob.Manifest, exists bool) error
	FetchBlobChunk(sha3 types.Hash) ([]byte, error)
	ReadBlobChunkRequest() (sha3 types.Hash, err error)
	SendBlobChunk(chunk []byte, exists bool) error
}

type blobProtocol struct {
	process.Process
	log.Logger

	blobStore   blob.Store
	transports  map[string]BlobTransport
	blobsNeeded *utils.Mailbox
}

const (
	BlobChunkSize = 1024 // @@TODO: tunable buffer size?
	ProtocolName  = "protoblob"
)

func NewBlobProtocol(transports []swarm.Transport, blobStore blob.Store) *blobProtocol {
	transportsMap := make(map[string]BlobTransport)
	for _, tpt := range transports {
		if tpt, is := tpt.(BlobTransport); is {
			transportsMap[tpt.Name()] = tpt
		}
	}
	return &blobProtocol{
		Process:     *process.New(ProtocolName),
		Logger:      log.NewLogger(ProtocolName),
		blobStore:   blobStore,
		transports:  transportsMap,
		blobsNeeded: utils.NewMailbox(0),
	}
}

func (bp *blobProtocol) Name() string {
	return ProtocolName
}

func (bp *blobProtocol) Start() error {
	bp.Process.Start()
	bp.blobStore.OnBlobsNeeded(func(blobs []blob.ID) {
		bp.blobsNeeded.Deliver(blobs)
	})

	bp.periodicallyFetchMissingBlobs()

	for _, tpt := range bp.transports {
		tpt.OnBlobManifestRequest(bp.handleBlobManifestRequest)
		tpt.OnBlobChunkRequest(bp.handleBlobChunkRequest)
	}
	return nil
}

func (bp *blobProtocol) ProvidersOfBlob(ctx context.Context, blobID blob.ID) <-chan BlobPeerConn {
	ch := make(chan BlobPeerConn)

	child := bp.Process.NewChild(ctx, "ProvidersOfBlob "+blobID.String())
	defer child.Autoclose()

	for _, tpt := range bp.transports {
		innerCh, err := tpt.ProvidersOfBlob(ctx, blobID)
		if err != nil {
			bp.Warnf("transport %v could not fetch providers of blob %v", tpt.Name(), blobID)
			continue
		}

		child.Go(tpt.Name(), func(ctx context.Context) {
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

	bp.Process.Go("ProvidersOfBlob "+blobID.String()+" (await completion)", func(ctx context.Context) {
		<-child.Done()
		close(ch)
	})

	return ch
}

func (bp *blobProtocol) periodicallyFetchMissingBlobs() {
	bp.Process.Go("periodicallyFetchMissingBlobs", func(ctx context.Context) {
		// ticker := utils.NewExponentialBackoffTicker(10*time.Second, 2*time.Minute) // @@TODO: configurable?
		ticker := time.NewTicker(10 * time.Second)
		// ticker.Start()
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-bp.blobsNeeded.Notify():
				blobsBlobs := bp.blobsNeeded.RetrieveAll()
				var allBlobs []blob.ID
				for _, blobs := range blobsBlobs {
					allBlobs = append(allBlobs, blobs.([]blob.ID)...)
				}
				bp.fetchBlobs(allBlobs)

				// case <-ticker.Tick():
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
		fetcher := newFetcher(blobID, 4, bp.blobStore, bp.ProvidersOfBlob)

		err := bp.Process.SpawnChild(nil, fetcher)
		if err != nil {
			bp.Errorf("error spawning blob fetcher (blobID: %v): %v", blobID, err)
			return
		}
	}
}

func (bp *blobProtocol) announceBlobs(blobIDs []blob.ID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	child := bp.Process.NewChild(ctx, "announceBlobs")

	for _, transport := range bp.transports {
		for _, blobID := range blobIDs {
			transport := transport
			blobID := blobID

			child.Go(blobID.String(), func(ctx context.Context) {
				err := transport.AnnounceBlob(ctx, blobID)
				if errors.Cause(err) == types.ErrUnimplemented {
					return
				} else if err != nil {
					bp.Warnf("error announcing blob %v over transport %v: %v", blobID, transport.Name(), err)
				}
			})
		}
	}
	child.Autoclose()
	<-child.Done()
}

func (bp *blobProtocol) handleBlobManifestRequest(blobID blob.ID, peer BlobPeerConn) {
	defer peer.Close()

	bp.Debugf("incoming blob manifest request for %v from %v", blobID, peer.DialInfo())

	manifest, err := bp.blobStore.Manifest(blobID)
	if errors.Cause(err) == types.Err404 {
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
	if errors.Cause(err) == types.Err404 {
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
