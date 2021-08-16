package protoauth

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/crypto"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm"
	"redwood.dev/types"
	"redwood.dev/utils"
)

//go:generate mockery --name AuthProtocol --output ./mocks/ --case=underscore
type AuthProtocol interface {
	process.Interface
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
	process.Process
	log.Logger

	keyStore   identity.KeyStore
	peerStore  swarm.PeerStore
	transports map[string]AuthTransport
}

func NewAuthProtocol(transports []swarm.Transport, keyStore identity.KeyStore, peerStore swarm.PeerStore) *authProtocol {
	transportsMap := make(map[string]AuthTransport)
	for _, tpt := range transports {
		if tpt, is := tpt.(AuthTransport); is {
			transportsMap[tpt.Name()] = tpt
		}
	}
	return &authProtocol{
		Process:    *process.New("AuthProtocol"),
		Logger:     log.NewLogger("auth proto"),
		transports: transportsMap,
		keyStore:   keyStore,
		peerStore:  peerStore,
	}
}

const ProtocolName = "auth"

func (ap *authProtocol) Name() string {
	return ProtocolName
}

func (ap *authProtocol) Start() error {
	ap.Process.Start()

	processPeersTask := NewProcessPeersTask(10*time.Second, ap, ap.peerStore, ap.transports)
	ap.peerStore.OnNewUnverifiedPeer(func(dialInfo swarm.PeerDialInfo) {
		processPeersTask.Enqueue()
	})
	err := ap.Process.SpawnChild(nil, processPeersTask)
	if err != nil {
		return err
	}

	for _, tpt := range ap.transports {
		tpt.OnChallengeIdentity(ap.handleChallengeIdentity)
	}
	return nil
}

func (ap *authProtocol) PeersClaimingAddress(ctx context.Context, address types.Address) <-chan AuthPeerConn {
	ch := make(chan AuthPeerConn)

	child := ap.Process.NewChild(ctx, "PeersClaimingAddress "+address.String())
	defer child.Autoclose()

	for _, tpt := range ap.transports {
		innerCh, err := tpt.PeersClaimingAddress(ctx, address)
		if err != nil {
			continue
		}

		child.Go(tpt.Name(), func(ctx context.Context) {
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
		})
	}

	ap.Process.Go("PeersClaimingAddress "+address.String()+" (await completion)", func(ctx context.Context) {
		<-child.Done()
		close(ch)
	})

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

type processPeersTask struct {
	process.PeriodicTask
	log.Logger
	authProto  AuthProtocol
	peerStore  swarm.PeerStore
	transports map[string]AuthTransport
}

func NewProcessPeersTask(
	interval time.Duration,
	authProto AuthProtocol,
	peerStore swarm.PeerStore,
	transports map[string]AuthTransport,
) *processPeersTask {
	t := &processPeersTask{
		Logger:     log.NewLogger("auth proto"),
		authProto:  authProto,
		peerStore:  peerStore,
		transports: transports,
	}
	t.PeriodicTask = *process.NewPeriodicTask("ProcessPeersTask", interval, t.processPeers)
	return t
}

func (t *processPeersTask) processPeers(ctx context.Context) {
	// Announce peers
	{
		allDialInfos := t.peerStore.AllDialInfos()

		for _, tpt := range t.transports {
			for _, peerDetails := range t.peerStore.PeersFromTransport(tpt.Name()) {
				if !peerDetails.Ready() {
					continue
				}

				if peerDetails.DialInfo().TransportName != tpt.Name() {
					continue
				}

				tpt := tpt
				peerDetails := peerDetails

				t.Process.Go("announce peers", func(ctx context.Context) {
					peerConn, err := tpt.NewPeerConn(ctx, peerDetails.DialInfo().DialAddr)
					if errors.Cause(err) == swarm.ErrPeerIsSelf {
						return
					} else if err != nil {
						t.Warnf("error creating new %v peerConn: %v", tpt.Name(), err)
						return
					}
					defer peerConn.Close()

					err = peerConn.EnsureConnected(ctx)
					if errors.Cause(err) == types.ErrConnection {
						return
					} else if err != nil {
						t.Warnf("error connecting to %v peerConn (%v): %v", tpt.Name(), peerDetails.DialInfo().DialAddr, err)
						return
					}

					peerConn.AnnouncePeers(ctx, allDialInfos)
					if err != nil {
						// t.Errorf("error writing to peerConn: %+v", err)
					}
				})
			}
		}
	}

	// Verify unverified peers
	{
		for _, unverifiedPeer := range t.peerStore.UnverifiedPeers() {
			if !unverifiedPeer.Ready() {
				continue
			}

			transport, exists := t.transports[unverifiedPeer.DialInfo().TransportName]
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

			t.Process.Go("verify unverified peers", func(ctx context.Context) {
				defer authPeerConn.Close()

				err := t.authProto.ChallengePeerIdentity(ctx, authPeerConn)
				if errors.Cause(err) == types.ErrConnection {
					// no-op
				} else if errors.Cause(err) == context.Canceled {
					// no-op
				} else if err != nil {
					t.Errorf("error verifying peerConn identity (%v): %v", authPeerConn.DialInfo(), err)
				}
			})
		}
	}

	// <-child.Done()
}
