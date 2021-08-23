package protoblob

import (
	"context"
	"io"
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
	process.Process
	log.Logger

	blobStore   blob.Store
	transports  map[string]BlobTransport
	blobsNeeded *utils.Mailbox
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
		Process:     *process.New("BlobProtocol"),
		Logger:      log.NewLogger("blob proto"),
		blobStore:   blobStore,
		transports:  transportsMap,
		blobsNeeded: utils.NewMailbox(0),
	}
}

const ProtocolName = "blob"

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
		tpt.OnBlobRequest(bp.handleBlobRequest)
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
		ticker := utils.NewExponentialBackoffTicker(10*time.Second, 2*time.Minute) // @@TODO: configurable?
		ticker.Start()
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
	})
}

func (bp *blobProtocol) fetchBlobs(blobs []blob.ID) {
	child := bp.Process.NewChild(context.Background(), "fetchBlobs")

	for _, blobID := range blobs {
		blobID := blobID
		child.Go(blobID.String(), func(ctx context.Context) {
			bp.fetchBlob(ctx, blobID)
		})
	}
	child.Autoclose()
	<-child.Done()
}

func (bp *blobProtocol) fetchBlob(ctx context.Context, blobID blob.ID) {
	ctxFindProviders, cancelFindProviders := context.WithTimeout(ctx, 15*time.Second)
	defer cancelFindProviders()

	bp.Warnf("looking for providers of blob %v", blobID)
	for peer := range bp.ProvidersOfBlob(ctxFindProviders, blobID) {
		if !peer.Ready() {
			continue
		}
		bp.Warnf("found provider of blob %v: %v", blobID, peer.DialInfo())

		var done bool
		func() {
			ctxConnect, cancelConnect := context.WithTimeout(ctx, 10*time.Second)
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
			bp.Process.Go("fetchBlob "+blobID.String()+" pipe", func(ctx context.Context) {
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
					bp.Warnf("received packet (len %v) for blob %v: %v", len(pkt.Data), blobID, peer.DialInfo())

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
			})

			sha1Hash, sha3Hash, err := bp.blobStore.StoreBlob(pr)
			if err != nil {
				bp.Errorf("could not store blob: %v", err)
				return
			}
			bp.Warnf("received entire blob %v: %v", blobID, peer.DialInfo())
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

func (bp *blobProtocol) handleBlobRequest(blobID blob.ID, peer BlobPeerConn) {
	defer peer.Close()

	bp.Debugf("incoming blob request: %v %v", blobID, peer.DialInfo())

	blobReader, blobLen, err := bp.blobStore.BlobReader(blobID)
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
	bp.Debugf("sent blob header: %v %v", blobID, peer.DialInfo())

	buf := make([]byte, BlobChunkSize)
	i := 0
	numPackets := blobLen / BlobChunkSize
	for {
		i++
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
		bp.Debugf("sent blob packet (%v / %v): %v %v", i, numPackets, blobID, peer.DialInfo())
		if end {
			break
		}
	}

	err = peer.SendBlobPacket(nil, true)
	if err != nil {
		bp.Errorf("%+v", errors.WithStack(err))
		return
	}
	bp.Debugf("sent blob footer: %v %v", blobID, peer.DialInfo())
}
