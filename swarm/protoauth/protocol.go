package protoauth

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/crypto"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/swarm"
	"redwood.dev/types"
	"redwood.dev/utils"
)

//go:generate mockery --name AuthProtocol --output ./mocks/ --case=underscore

type AuthProtocol interface {
	swarm.Protocol
	PeersClaimingAddress(ctx context.Context, address types.Address) <-chan AuthPeerConn
	ChallengePeerIdentity(ctx context.Context, peerConn AuthPeerConn) (err error)
}

//go:generate mockery --name AuthTransport --output ./mocks/ --case=underscore
type AuthTransport interface {
	swarm.Transport
	PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan AuthPeerConn, error)
	OnChallengeIdentity(handler ChallengeIdentityCallback)
}

//go:generate mockery --name AuthPeerConn --output ./mocks/ --case=underscore
type AuthPeerConn interface {
	swarm.PeerConn
	ChallengeIdentity(challengeMsg ChallengeMsg) error
	RespondChallengeIdentity(verifyAddressResponse []ChallengeIdentityResponse) error
	ReceiveChallengeIdentityResponse() ([]ChallengeIdentityResponse, error)
}

type authProtocol struct {
	log.Logger

	keyStore         identity.KeyStore
	peerStore        swarm.PeerStore
	transports       map[string]AuthTransport
	processPeersTask *utils.PeriodicTask

	chStop chan struct{}
	wgDone sync.WaitGroup
}

func NewAuthProtocol(transports []swarm.Transport, keyStore identity.KeyStore, peerStore swarm.PeerStore) *authProtocol {
	transportsMap := make(map[string]AuthTransport)
	for _, tpt := range transports {
		if tpt, is := tpt.(AuthTransport); is {
			transportsMap[tpt.Name()] = tpt
		}
	}
	return &authProtocol{
		Logger:     log.NewLogger("auth proto"),
		transports: transportsMap,
		keyStore:   keyStore,
		peerStore:  peerStore,
		chStop:     make(chan struct{}),
	}
}

const ProtocolName = "auth"

func (ap *authProtocol) Name() string {
	return ProtocolName
}

func (ap *authProtocol) Start() {
	ap.wgDone.Add(1)
	ap.peerStore.OnNewUnverifiedPeer(func(dialInfo swarm.PeerDialInfo) {
		ap.processPeersTask.Enqueue()
	})
	ap.processPeersTask = utils.NewPeriodicTask(10*time.Second, ap.processPeers)
	for _, tpt := range ap.transports {
		tpt.OnChallengeIdentity(ap.handleChallengeIdentity)
	}
}

func (ap *authProtocol) Close() {
	ap.processPeersTask.Close()
	close(ap.chStop)
	ap.wgDone.Done()
	ap.wgDone.Wait()
}

func (ap *authProtocol) PeersClaimingAddress(ctx context.Context, address types.Address) <-chan AuthPeerConn {
	ctx, cancel := utils.CombinedContext(ctx, ap.chStop)

	var wg sync.WaitGroup
	ch := make(chan AuthPeerConn)
	for _, tpt := range ap.transports {
		innerCh, err := tpt.PeersClaimingAddress(ctx, address)
		if err != nil {
			continue
		}

		wg.Add(1)
		ap.wgDone.Add(1)
		go func() {
			defer wg.Done()
			defer ap.wgDone.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case peerConn, open := <-innerCh:
					if !open {
						return
					}

					select {
					case <-ctx.Done():
						return
					case ch <- peerConn:
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

func (ap *authProtocol) ChallengePeerIdentity(ctx context.Context, peerConn AuthPeerConn) (err error) {
	defer utils.WithStack(&err)

	err = peerConn.EnsureConnected(ctx)
	if err != nil {
		return err
	}

	challengeMsg, err := GenerateChallengeMsg()
	if err != nil {
		return err
	}

	err = peerConn.ChallengeIdentity(challengeMsg)
	if err != nil {
		return err
	}

	resp, err := peerConn.ReceiveChallengeIdentityResponse()
	if err != nil {
		return err
	}

	for _, proof := range resp {
		sigpubkey, err := crypto.RecoverSigningPubkey(types.HashBytes(challengeMsg), proof.Signature)
		if err != nil {
			return err
		}
		encpubkey := crypto.AsymEncPubkeyFromBytes(proof.AsymEncPubkey)

		ap.peerStore.AddVerifiedCredentials(peerConn.DialInfo(), sigpubkey.Address(), sigpubkey, encpubkey)
	}

	return nil
}

func (ap *authProtocol) handleChallengeIdentity(challengeMsg ChallengeMsg, peerConn AuthPeerConn) error {
	defer peerConn.Close()

	publicIdentities, err := ap.keyStore.PublicIdentities()
	if err != nil {
		ap.Errorf("error fetching public identities from key store: %v", err)
		return err
	}

	var responses []ChallengeIdentityResponse
	for _, identity := range publicIdentities {
		sig, err := ap.keyStore.SignHash(identity.Address(), types.HashBytes(challengeMsg))
		if err != nil {
			ap.Errorf("error signing hash: %v", err)
			return err
		}
		responses = append(responses, ChallengeIdentityResponse{
			Signature:     sig,
			AsymEncPubkey: identity.AsymEncKeypair.AsymEncPubkey.Bytes(),
		})
	}
	return peerConn.RespondChallengeIdentity(responses)
}

func (ap *authProtocol) processPeers(ctx context.Context) {
	ctx, cancel := utils.CombinedContext(ctx, ap.chStop)
	defer cancel()

	wg := utils.NewWaitGroupChan(ctx)
	defer wg.Close()

	wg.Add(1)

	// Announce peers
	{
		allDialInfos := ap.peerStore.AllDialInfos()

		for _, tpt := range ap.transports {
			for _, peerDetails := range ap.peerStore.PeersFromTransport(tpt.Name()) {
				if !peerDetails.Ready() {
					continue
				}

				if peerDetails.DialInfo().TransportName != tpt.Name() {
					continue
				}

				tpt := tpt
				peerDetails := peerDetails

				wg.Add(1)
				ap.wgDone.Add(1)
				go func() {
					defer wg.Done()
					defer ap.wgDone.Done()

					peerConn, err := tpt.NewPeerConn(ctx, peerDetails.DialInfo().DialAddr)
					if errors.Cause(err) == swarm.ErrPeerIsSelf {
						return
					} else if err != nil {
						ap.Warnf("error creating new %v peerConn: %v", tpt.Name(), err)
						return
					}
					defer peerConn.Close()

					err = peerConn.EnsureConnected(ctx)
					if errors.Cause(err) == types.ErrConnection {
						return
					} else if err != nil {
						ap.Warnf("error connecting to %v peerConn (%v): %v", tpt.Name(), peerDetails.DialInfo().DialAddr, err)
						return
					}

					peerConn.AnnouncePeers(ctx, allDialInfos)
					if err != nil {
						// t.Errorf("error writing to peerConn: %+v", err)
					}
				}()
			}
		}
	}

	// Verify unverified peers
	{
		unverifiedPeers := ap.peerStore.UnverifiedPeers()

		for _, unverifiedPeer := range unverifiedPeers {
			if !unverifiedPeer.Ready() {
				continue
			}

			transport, exists := ap.transports[unverifiedPeer.DialInfo().TransportName]
			if !exists {
				// Unsupported transport
				continue
			}

			peerConn, err := transport.NewPeerConn(ctx, unverifiedPeer.DialInfo().DialAddr)
			if errors.Cause(err) == swarm.ErrPeerIsSelf {
				continue
			} else if errors.Cause(err) == types.ErrConnection {
				continue
			} else if err != nil {
				continue
			} else if !peerConn.Ready() {
				continue
			}

			authPeerConn, is := peerConn.(AuthPeerConn)
			if !is {
				continue
			}

			wg.Add(1)
			ap.wgDone.Add(1)
			go func() {
				defer wg.Done()
				defer ap.wgDone.Done()
				defer authPeerConn.Close()

				err := ap.ChallengePeerIdentity(ctx, authPeerConn)
				if errors.Cause(err) == types.ErrConnection {
					// no-op
				} else if errors.Cause(err) == context.Canceled {
					// no-op
				} else if err != nil {
					ap.Errorf("error verifying peerConn identity (%v): %v", authPeerConn.DialInfo(), err)
				}
			}()
		}
	}

	wg.Done()
	<-wg.Wait()
}
