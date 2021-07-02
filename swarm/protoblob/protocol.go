package protoblob

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/blob"
	"redwood.dev/log"
	"redwood.dev/swarm"
	"redwood.dev/types"
	"redwood.dev/utils"
)

//go:generate mockery --name BlobProtocol --output ./mocks/ --case=underscore
type BlobProtocol interface {
	swarm.Protocol
	ProvidersOfBlob(ctx context.Context, blobID blob.ID) <-chan BlobPeerConn
}

//go:generate mockery --name BlobTransport --output ./mocks/ --case=underscore
type BlobTransport interface {
	swarm.Transport
	ProvidersOfBlob(ctx context.Context, blobID blob.ID) (<-chan BlobPeerConn, error)
	AnnounceBlob(ctx context.Context, blobID blob.ID) error
	OnBlobRequest(handler func(blobID blob.ID, peer BlobPeerConn))
}

//go:generate mockery --name BlobPeerConn --output ./mocks/ --case=underscore
type BlobPeerConn interface {
	swarm.PeerConn
	FetchBlob(blobID blob.ID) error
	SendBlobHeader(haveBlob bool) error
	SendBlobPacket(data []byte, end bool) error
	ReceiveBlobHeader() (FetchBlobResponseHeader, error)
	ReceiveBlobPacket() (FetchBlobResponseBody, error)
}

type blobProtocol struct {
	log.Logger
	blobStore   blob.Store
	transports  map[string]BlobTransport
	blobsNeeded *utils.Mailbox
	chStop      chan struct{}
	chDone      chan struct{}
}

const (
	BlobChunkSize = 1024 // @@TODO: tunable buffer size?
)

func NewBlobProtocol(transports []swarm.Transport, blobStore blob.Store) *blobProtocol {
	transportsMap := make(map[string]BlobTransport)
	for _, tpt := range transports {
		if tpt, is := tpt.(BlobTransport); is {
			transportsMap[tpt.Name()] = tpt
		}
	}
	return &blobProtocol{
		Logger:      log.NewLogger("blob proto"),
		blobStore:   blobStore,
		transports:  transportsMap,
		blobsNeeded: utils.NewMailbox(0),
		chStop:      make(chan struct{}),
		chDone:      make(chan struct{}),
	}
}

const ProtocolName = "blob"

func (bp *blobProtocol) Name() string {
	return ProtocolName
}

func (bp *blobProtocol) Start() {
	bp.blobStore.OnBlobsNeeded(func(blobs []blob.ID) {
		bp.blobsNeeded.Deliver(blobs)
	})

	go bp.periodicallyFetchMissingBlobs()

	for _, tpt := range bp.transports {
		tpt.OnBlobRequest(bp.handleBlobRequest)
	}
}

func (bp *blobProtocol) Close() {
	close(bp.chStop)
	<-bp.chDone
}

func (bp *blobProtocol) ProvidersOfBlob(ctx context.Context, blobID blob.ID) <-chan BlobPeerConn {
	ctx, cancel := utils.CombinedContext(ctx, bp.chStop)

	var wg sync.WaitGroup
	ch := make(chan BlobPeerConn)
	for _, tpt := range bp.transports {
		innerCh, err := tpt.ProvidersOfBlob(ctx, blobID)
		if err != nil {
			bp.Warnf("transport %v could not fetch providers of blob %v", tpt.Name(), blobID)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
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
		}()
	}

	go func() {
		defer cancel()
		defer close(ch)
		wg.Wait()
	}()

	return ch
}

func (bp *blobProtocol) periodicallyFetchMissingBlobs() {
	ticker := utils.NewExponentialBackoffTicker(10*time.Second, 2*time.Minute) // @@TODO: configurable?
	ticker.Start()
	defer ticker.Stop()

	for {
		select {
		case <-bp.chStop:
			return

		case <-bp.blobsNeeded.Notify():
			blobsBlobs := bp.blobsNeeded.RetrieveAll()
			var allBlobs []blob.ID
			for _, blobs := range blobsBlobs {
				allBlobs = append(allBlobs, blobs.([]blob.ID)...)
			}
			bp.fetchBlobs(allBlobs)

		case <-ticker.Tick():
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
}

func (bp *blobProtocol) fetchBlobs(blobs []blob.ID) {
	var wg sync.WaitGroup
	for _, blobID := range blobs {
		wg.Add(1)
		blobID := blobID
		go func() {
			defer wg.Done()
			bp.fetchBlob(context.Background(), blobID)
		}()
	}
	wg.Wait()
}

func (bp *blobProtocol) fetchBlob(ctx context.Context, blobID blob.ID) {
	ctxFindProviders, cancelFindProviders := utils.CombinedContext(ctx, bp.chStop, 15*time.Second)
	defer cancelFindProviders()

	for peer := range bp.ProvidersOfBlob(ctxFindProviders, blobID) {
		if !peer.Ready() {
			continue
		}

		var done bool
		func() {
			ctxConnect, cancelConnect := utils.CombinedContext(bp.chStop, 10*time.Second)
			defer cancelConnect()

			err := peer.EnsureConnected(ctxConnect)
			if err != nil {
				bp.Errorf("error connecting to peer: %v", err)
				return
			}
			cancelConnect()
			defer peer.Close()

			err = peer.FetchBlob(blobID)
			if err != nil {
				bp.Errorf("error writing to peer: %v", err)
				return
			}

			header, err := peer.ReceiveBlobHeader()
			if err != nil {
				bp.Errorf("error reading from peer: %v", err)
				return
			} else if header.Missing {
				bp.Errorf("peer doesn't have blob: %v", err)
				return
			}

			pr, pw := io.Pipe()
			go func() {
				var err error
				defer func() { pw.CloseWithError(err) }()

				for {
					select {
					case <-ctx.Done():
						err = ctx.Err()
						return
					default:
					}

					pkt, err := peer.ReceiveBlobPacket()
					if err != nil {
						bp.Errorf("error receiving blob from peer: %v", err)
						return
					} else if pkt.End {
						return
					}

					var n int
					n, err = pw.Write(pkt.Data)
					if err != nil {
						bp.Errorf("error receiving blob from peer: %v", err)
						return
					} else if n < len(pkt.Data) {
						err = io.ErrUnexpectedEOF
						return
					}
				}
			}()

			sha1Hash, sha3Hash, err := bp.blobStore.StoreBlob(pr)
			if err != nil {
				bp.Errorf("could not store blob: %v", err)
				return
			}
			// @@TODO: check stored blobHash against the one we requested

			bp.announceBlobs([]blob.ID{
				{HashAlg: types.SHA1, Hash: sha1Hash},
				{HashAlg: types.SHA3, Hash: sha3Hash},
			})
			done = true
		}()
		if done {
			return
		}
	}
}

func (bp *blobProtocol) announceBlobs(blobIDs []blob.ID) {
	ctx, cancel := utils.CombinedContext(bp.chStop, 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(len(blobIDs) * len(bp.transports))

	for _, transport := range bp.transports {
		for _, blobID := range blobIDs {
			transport := transport
			blobID := blobID

			go func() {
				defer wg.Done()

				err := transport.AnnounceBlob(ctx, blobID)
				if errors.Cause(err) == types.ErrUnimplemented {
					return
				} else if err != nil {
					bp.Warnf("error announcing blob %v over transport %v: %v", blobID, transport.Name(), err)
				}
			}()
		}
	}
	wg.Wait()
}

func (bp *blobProtocol) handleBlobRequest(blobID blob.ID, peer BlobPeerConn) {
	defer peer.Close()

	blobReader, _, err := bp.blobStore.BlobReader(blobID)
	if err != nil {
		err = peer.SendBlobHeader(false)
		if err != nil {
			bp.Errorf("while sending blob header: %v", err)
		}
		return
	}
	defer blobReader.Close()

	err = peer.SendBlobHeader(true)
	if err != nil {
		bp.Errorf("%+v", errors.WithStack(err))
		return
	}

	buf := make([]byte, BlobChunkSize)
	for {
		var end bool
		n, err := io.ReadFull(blobReader, buf)
		if err == io.EOF {
			break
		} else if err == io.ErrUnexpectedEOF {
			buf = buf[:n]
			end = true
		} else if err != nil {
			bp.Errorf("%+v", err)
			return
		}

		err = peer.SendBlobPacket(buf, false)
		if err != nil {
			bp.Errorf("%+v", errors.WithStack(err))
			return
		}
		if end {
			break
		}
	}

	err = peer.SendBlobPacket(nil, true)
	if err != nil {
		bp.Errorf("%+v", errors.WithStack(err))
		return
	}
}
