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
	"redwood.dev/utils/authutils"
	. "redwood.dev/utils/generics"
)

//go:generate mockery --name AuthProtocol --output ./mocks/ --case=underscore
type AuthProtocol interface {
	process.Interface
	ChallengePeerIdentity(ctx context.Context, peerConn AuthPeerConn)
}

//go:generate mockery --name AuthTransport --output ./mocks/ --case=underscore
type AuthTransport interface {
	swarm.Transport
	OnIncomingIdentityChallenge(handler IncomingIdentityChallengeCallback)
	OnIncomingIdentityChallengeRequest(handler IncomingIdentityChallengeRequestCallback)
	OnIncomingIdentityChallengeResponse(handler IncomingIdentityChallengeResponseCallback)
	OnIncomingUcan(handler IncomingUcanCallback)
}

//go:generate mockery --name AuthPeerConn --output ./mocks/ --case=underscore
type AuthPeerConn interface {
	swarm.PeerConn
	// RequestIdentityChallenge() (crypto.ChallengeMsg, error)
	ChallengeIdentity(ctx context.Context, challengeMsg crypto.ChallengeMsg) error
	RespondChallengeIdentity(ctx context.Context, response []ChallengeIdentityResponse) error
	SendUcan(ctx context.Context, ucan string) error
}

type authProtocol struct {
	swarm.BaseProtocol[AuthTransport, AuthPeerConn]

	keyStore   identity.KeyStore
	peerStore  swarm.PeerStore
	poolWorker process.PoolWorker

	jwtSecret             []byte
	outstandingChallenges SyncSet[string]
}

func NewAuthProtocol(transports []swarm.Transport, keyStore identity.KeyStore, peerStore swarm.PeerStore, jwtSecret []byte) *authProtocol {
	transportsMap := make(map[string]AuthTransport)
	for _, tpt := range transports {
		if tpt, is := tpt.(AuthTransport); is {
			transportsMap[tpt.Name()] = tpt
		}
	}

	ap := &authProtocol{
		BaseProtocol: swarm.BaseProtocol[AuthTransport, AuthPeerConn]{
			Process:    *process.New(ProtocolName),
			Logger:     log.NewLogger(ProtocolName),
			Transports: transportsMap,
		},
		keyStore:              keyStore,
		peerStore:             peerStore,
		jwtSecret:             jwtSecret,
		outstandingChallenges: NewSyncSet[string](nil),
	}

	ap.poolWorker = process.NewPoolWorker("pool worker", 4, process.NewStaticScheduler(5*time.Second, 10*time.Second))

	ap.peerStore.OnNewUnverifiedPeer(func(dialInfo swarm.PeerDialInfo) {
		ap.Debugf("new unverified peer: %+v", dialInfo)
		ap.poolWorker.Add(verifyPeer{dialInfo, ap})
	})

	for _, tpt := range ap.Transports {
		ap.Infof("registering %v", tpt.Name())
		tpt.OnIncomingIdentityChallenge(ap.handleIncomingIdentityChallenge)
		tpt.OnIncomingIdentityChallengeRequest(ap.handleIncomingIdentityChallengeRequest)
		tpt.OnIncomingIdentityChallengeResponse(ap.handleIncomingIdentityChallengeResponse)
		tpt.OnIncomingUcan(ap.handleIncomingUcan)
	}

	go func() {
		for {
			for _, pd := range ap.peerStore.UnverifiedPeers() {
				for dialInfo := range pd.Endpoints() {
					ap.poolWorker.Add(verifyPeer{dialInfo, ap})
					ap.poolWorker.ForceRetry(verifyPeer{dialInfo, ap})
				}
			}

			time.Sleep(10 * time.Second)
		}
	}()

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
			for _, device := range ap.peerStore.UnverifiedPeers() {
				for dialInfo := range device.Endpoints() {
					ap.poolWorker.Add(verifyPeer{dialInfo, ap})
				}
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
	ap.Infof("auth protocol shutting down")
	return ap.Process.Close()
}

func (ap *authProtocol) ChallengePeerIdentity(ctx context.Context, peerConn AuthPeerConn) {
	ap.poolWorker.Add(verifyPeer{peerConn.DialInfo(), ap})
}

func (ap *authProtocol) handleIncomingIdentityChallenge(challengeMsg crypto.ChallengeMsg, peerConn AuthPeerConn) error {
	ap.poolWorker.Add(verifyPeer{peerConn.DialInfo(), ap})
	ap.poolWorker.ForceRetry(verifyPeer{peerConn.DialInfo(), ap})

	publicIdentities, err := ap.keyStore.PublicIdentities()
	if err != nil {
		ap.Errorf("error fetching public identities from key store: %v", err)
		return err
	}

	var responses []ChallengeIdentityResponse
	for _, id := range publicIdentities {
		resp := ChallengeIdentityResponse{Challenge: challengeMsg, AsymEncPubkey: *id.AsymEncKeypair.AsymEncPubkey}

		sig, err := id.SignHash(resp.Hash())
		if err != nil {
			ap.Errorf("error signing hash: %v", err)
			return err
		}
		resp.Signature = sig
		responses = append(responses, resp)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = peerConn.RespondChallengeIdentity(ctx, responses)
	if err != nil {
		ap.Errorf("error responding to identity challenge: %v", err)
		return err
	}
	return nil
}

func (ap *authProtocol) handleIncomingIdentityChallengeRequest(peerConn AuthPeerConn) error {
	challengeMsg, err := crypto.GenerateChallengeMsg()
	if err != nil {
		return err
	}
	ap.outstandingChallenges.Add(challengeMsg.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return peerConn.ChallengeIdentity(ctx, challengeMsg)
}

func (ap *authProtocol) handleIncomingIdentityChallengeResponse(signatures []ChallengeIdentityResponse, peerConn AuthPeerConn) error {
	addresses := NewSet[types.Address](nil)
	challengesToRemove := NewSet[string](nil)
	for _, proof := range signatures {
		if !ap.outstandingChallenges.Contains(proof.Challenge.String()) {
			continue
		}

		sigpubkey, err := crypto.RecoverSigningPubkey(proof.Hash(), proof.Signature)
		if err != nil {
			ap.Warnw("bad challenge response", "peer", peerConn.DialInfo(), "err", err)
			continue
		}
		ap.peerStore.AddVerifiedCredentials(peerConn.DialInfo(), peerConn.DeviceUniqueID(), sigpubkey.Address(), sigpubkey, &proof.AsymEncPubkey)
		addresses.Add(sigpubkey.Address())
		challengesToRemove.Add(proof.Challenge.String())
	}

	ucan := authutils.Ucan{
		Addresses: addresses,
		IssuedAt:  time.Now(),
	}
	ucanStr, err := ucan.SignedString(ap.jwtSecret)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = peerConn.SendUcan(ctx, ucanStr)
	if err != nil {
		return err
	}

	for challenge := range challengesToRemove {
		ap.outstandingChallenges.Remove(challenge)
	}
	ap.Successw("authenticated", "peer", peerConn.DialInfo(), "addresses", peerConn.Addresses().Slice())
	return nil
}

func (ap *authProtocol) handleIncomingUcan(ucan string, peerConn AuthPeerConn) {
	ap.keyStore.SaveExtraUserData("ucan:"+peerConn.DeviceUniqueID(), ucan)
}

type verifyPeer struct {
	dialInfo  swarm.PeerDialInfo
	authProto *authProtocol
}

var _ process.PoolWorkerItem = verifyPeer{}

func (t verifyPeer) ID() process.PoolUniqueID { return t }

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

	transport, exists := t.authProto.Transports[unverifiedPeer.DialInfo().TransportName]
	if !exists {
		// Unsupported transport
		return false
	}

	ok := <-transport.AwaitReady(ctx)
	if !ok {
		return true
	}

	peerConn, err := transport.NewPeerConn(ctx, unverifiedPeer.DialInfo().DialAddr)
	if errors.Cause(err) == swarm.ErrPeerIsSelf {
		return false
	} else if errors.Cause(err) == errors.ErrConnection {
		return true
	} else if err != nil {
		return true
	}
	defer peerConn.Close()

	authPeerConn, is := peerConn.(AuthPeerConn)
	if !is {
		return true
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err = authPeerConn.EnsureConnected(ctx)
	if err != nil {
		return true
	}
	challengeMsg, err := crypto.GenerateChallengeMsg()
	if err != nil {
		return true
	}
	t.authProto.outstandingChallenges.Add(challengeMsg.String())

	err = authPeerConn.ChallengeIdentity(ctx, challengeMsg)
	if err != nil {
		// t.authProto.Errorw("failed to challenge peer identity", "peer", authPeerConn.DialInfo(), "err", err)
		return true
	}
	return false
}
