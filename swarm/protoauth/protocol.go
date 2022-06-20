package protoauth

import (
	"context"
	"time"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm"
	"redwood.dev/types"
)

//go:generate mockery --name AuthProtocol --output ./mocks/ --case=underscore
type AuthProtocol interface {
	process.Interface
	ChallengePeerIdentity(ctx context.Context, peerConn AuthPeerConn) (err error)
}

//go:generate mockery --name AuthTransport --output ./mocks/ --case=underscore
type AuthTransport interface {
	swarm.Transport
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
	poolWorker process.PoolWorker
}

func NewAuthProtocol(transports []swarm.Transport, keyStore identity.KeyStore, peerStore swarm.PeerStore) *authProtocol {
	transportsMap := make(map[string]AuthTransport)
	for _, tpt := range transports {
		if tpt, is := tpt.(AuthTransport); is {
			transportsMap[tpt.Name()] = tpt
		}
	}

	ap := &authProtocol{
		Process:    *process.New("AuthProtocol"),
		Logger:     log.NewLogger(ProtocolName),
		transports: transportsMap,
		keyStore:   keyStore,
		peerStore:  peerStore,
	}

	ap.poolWorker = process.NewPoolWorker("pool worker", 16, process.NewStaticScheduler(5*time.Second, 10*time.Second))

	ap.peerStore.OnNewUnverifiedPeer(func(dialInfo swarm.PeerDialInfo) {
		ap.Debugf("new unverified peer: %+v", dialInfo)
		// @@TODO: the following line causes some kind of infinite loop when > 1 peer is online
		// announcePeersTask.Enqueue()
		ap.poolWorker.Add(verifyPeer{dialInfo, ap})
	})

	for _, tpt := range ap.transports {
		ap.Infof(0, "registering %v", tpt.Name())
		tpt.OnChallengeIdentity(ap.handleChallengeIdentity)
	}

	return ap
}

const ProtocolName = "protoauth"

func (ap *authProtocol) Name() string {
	return ProtocolName
}

func (ap *authProtocol) Start() error {
	err := ap.Process.Start()
	if err != nil {
		return err
	}

	err = ap.Process.SpawnChild(nil, ap.poolWorker)
	if err != nil {
		return err
	}

	ap.Process.Go(nil, "periodically verify unverified peers", func(ctx context.Context) {
		for {
			for _, dialInfo := range ap.peerStore.UnverifiedPeers() {
				ap.poolWorker.Add(verifyPeer{dialInfo, ap})
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	})

	return nil
}

func (ap *authProtocol) Close() error {
	ap.Infof(0, "auth protocol shutting down")
	return ap.Process.Close()
}

func (ap *authProtocol) ChallengePeerIdentity(ctx context.Context, peerConn AuthPeerConn) (err error) {
	defer errors.AddStack(&err)

	if !peerConn.Ready() || !peerConn.Dialable() {
		return errors.Wrapf(swarm.ErrUnreachable, "peer: %v", peerConn.DialInfo())
	}

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

		ap.peerStore.AddVerifiedCredentials(peerConn.DialInfo(), peerConn.DeviceUniqueID(), sigpubkey.Address(), sigpubkey, encpubkey)
	}
	return nil
}

func (ap *authProtocol) handleChallengeIdentity(challengeMsg ChallengeMsg, peerConn AuthPeerConn) error {
	defer peerConn.Close()

	ap.poolWorker.Add(verifyPeer{peerConn.DialInfo(), ap})
	ap.poolWorker.ForceRetry(verifyPeer{peerConn.DialInfo(), ap})

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

	err = peerConn.RespondChallengeIdentity(responses)
	if err != nil {
		ap.Errorf("error responding to identity challenge: %v", err)
		return err
	}
	return nil
}

type verifyPeer struct {
	dialInfo  swarm.PeerDialInfo
	authProto *authProtocol
}

var _ process.PoolWorkerItem = verifyPeer{}

func (t verifyPeer) BlacklistUniqueID() process.PoolUniqueID    { return t }
func (t verifyPeer) RetryUniqueID() process.PoolUniqueID        { return t }
func (t verifyPeer) DedupeActiveUniqueID() process.PoolUniqueID { return t }
func (t verifyPeer) ID() process.PoolUniqueID                   { return t }

func (t verifyPeer) Work(ctx context.Context) (retry bool) {
	unverifiedPeer := t.authProto.peerStore.PeerEndpoint(t.dialInfo)
	if unverifiedPeer == nil {
		return true
	}

	if !unverifiedPeer.Ready() {
		return true
	} else if !unverifiedPeer.Dialable() {
		return false
	}

	transport, exists := t.authProto.transports[unverifiedPeer.DialInfo().TransportName]
	if !exists {
		// Unsupported transport
		return false
	}

	peerConn, err := transport.NewPeerConn(ctx, unverifiedPeer.DialInfo().DialAddr)
	if errors.Cause(err) == swarm.ErrPeerIsSelf {
		return false
	} else if errors.Cause(err) == errors.ErrConnection {
		return true
	} else if err != nil {
		return true
	}

	authPeerConn, is := peerConn.(AuthPeerConn)
	if !is {
		return false
	}
	defer authPeerConn.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err = authPeerConn.EnsureConnected(ctx)
	if err != nil {
		return true
	}

	err = t.authProto.ChallengePeerIdentity(ctx, authPeerConn)
	if errors.Cause(err) == errors.ErrConnection {
		// no-op
		return true
	} else if errors.Cause(err) == context.Canceled {
		// no-op
		return true
	} else if err != nil {
		return true
	}
	t.authProto.Successf("authenticated with %v (addresses=%v)", t.dialInfo, authPeerConn.Addresses())
	return false
}
