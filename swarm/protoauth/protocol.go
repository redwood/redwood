package protoauth

import (
	"context"
	"strings"
	"time"

	"redwood.dev/crypto"
	"redwood.dev/errors"
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
	return &authProtocol{
		Process:    *process.New("AuthProtocol"),
		Logger:     log.NewLogger(ProtocolName),
		transports: transportsMap,
		keyStore:   keyStore,
		peerStore:  peerStore,
	}
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

	ap.poolWorker = process.NewPoolWorker("pool worker", 16, process.NewStaticScheduler(5*time.Second, 10*time.Second))
	err = ap.Process.SpawnChild(nil, ap.poolWorker)
	if err != nil {
		return err
	}

	announcePeersTask := NewAnnouncePeersTask(10*time.Second, ap, ap.peerStore, ap.transports)
	ap.peerStore.OnNewUnverifiedPeer(func(dialInfo swarm.PeerDialInfo) {
		ap.Debugf("new unverified peer: %+v", dialInfo)
		// @@TODO: the following line causes some kind of infinite loop when > 1 peer is online
		// announcePeersTask.Enqueue()
		ap.poolWorker.Add(verifyPeer{dialInfo, ap})
	})
	err = ap.Process.SpawnChild(nil, announcePeersTask)
	if err != nil {
		return err
	}

	go func() {
		for {
			for _, dialInfo := range ap.peerStore.UnverifiedPeers() {
				ap.poolWorker.Add(verifyPeer{dialInfo, ap})
			}
			time.Sleep(5 * time.Second)
		}
	}()

	for _, tpt := range ap.transports {
		ap.Infof(0, "registering %v", tpt.Name())
		tpt.OnChallengeIdentity(ap.handleChallengeIdentity)
	}
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

	if strings.Contains(peerConn.DialInfo().String(), "fkU") {
		ap.Warnf("ChallengePeerIdentity 111")
	}

	err = peerConn.EnsureConnected(ctx)
	if err != nil {
		return err
	}
	if strings.Contains(peerConn.DialInfo().String(), "fkU") {
		ap.Warnf("ChallengePeerIdentity 222")
	}

	challengeMsg, err := GenerateChallengeMsg()
	if err != nil {
		return err
	}
	if strings.Contains(peerConn.DialInfo().String(), "fkU") {
		ap.Warnf("ChallengePeerIdentity 333")
	}

	err = peerConn.ChallengeIdentity(challengeMsg)
	if err != nil {
		return err
	}
	if strings.Contains(peerConn.DialInfo().String(), "fkU") {
		ap.Warnf("ChallengePeerIdentity 444")
	}

	resp, err := peerConn.ReceiveChallengeIdentityResponse()
	if err != nil {
		return err
	}
	if strings.Contains(peerConn.DialInfo().String(), "fkU") {
		ap.Warnf("ChallengePeerIdentity 555")
	}

	for _, proof := range resp {
		sigpubkey, err := crypto.RecoverSigningPubkey(types.HashBytes(challengeMsg), proof.Signature)
		if err != nil {
			return err
		}
		encpubkey := crypto.AsymEncPubkeyFromBytes(proof.AsymEncPubkey)

		if strings.Contains(peerConn.DialInfo().String(), "fkU") {
			ap.Warnf("ChallengePeerIdentity 6")
		}
		ap.peerStore.AddVerifiedCredentials(peerConn.DialInfo(), peerConn.DeviceUniqueID(), sigpubkey.Address(), sigpubkey, encpubkey)
	}
	if strings.Contains(peerConn.DialInfo().String(), "fkU") {
		ap.Warnf("ChallengePeerIdentity 777")
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

type announcePeersTask struct {
	process.PeriodicTask
	log.Logger
	authProto  AuthProtocol
	peerStore  swarm.PeerStore
	transports map[string]AuthTransport
}

func NewAnnouncePeersTask(
	interval time.Duration,
	authProto AuthProtocol,
	peerStore swarm.PeerStore,
	transports map[string]AuthTransport,
) *announcePeersTask {
	t := &announcePeersTask{
		Logger:     log.NewLogger(ProtocolName),
		authProto:  authProto,
		peerStore:  peerStore,
		transports: transports,
	}
	t.PeriodicTask = *process.NewPeriodicTask("AnnouncePeersTask", utils.NewStaticTicker(interval), t.announcePeers)
	return t
}

func (t *announcePeersTask) announcePeers(ctx context.Context) {
	// Announce peers
	{
		var allDialInfos []swarm.PeerDialInfo
		for dialInfo := range t.peerStore.AllDialInfos() {
			allDialInfos = append(allDialInfos, dialInfo)
		}

		for _, tpt := range t.transports {
			for _, peerDetails := range t.peerStore.PeersFromTransport(tpt.Name()) {
				if !peerDetails.Ready() || !peerDetails.Dialable() {
					continue
				} else if peerDetails.DialInfo().TransportName != tpt.Name() {
					continue
				}

				tpt := tpt
				peerDetails := peerDetails

				t.Process.Go(nil, "announce peers", func(ctx context.Context) {
					peerConn, err := tpt.NewPeerConn(ctx, peerDetails.DialInfo().DialAddr)
					if errors.Cause(err) == swarm.ErrPeerIsSelf {
						return
					} else if err != nil {
						t.Warnf("error creating new %v peerConn: %v", tpt.Name(), err)
						return
					}
					defer peerConn.Close()

					err = peerConn.EnsureConnected(ctx)
					if errors.Cause(err) == errors.ErrConnection {
						return
					} else if err != nil {
						t.Warnf("error connecting to %v peerConn (%v): %v", tpt.Name(), peerDetails.DialInfo().DialAddr, err)
						return
					}

					err = peerConn.AnnouncePeers(ctx, allDialInfos)
					if err != nil {
						// t.Errorf("error writing to peerConn: %+v", err)
					}
				})
			}
		}
	}
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
	only := func(msg string, args ...interface{}) {
		// if strings.Contains(t.dialInfo.DialAddr, "/p2p-circuit") {
		// t.authProto.Debugf(msg, args...)
		// }
	}

	only("verifyPeer %v", t.dialInfo)

	unverifiedPeer := t.authProto.peerStore.PeerEndpoint(t.dialInfo)
	if unverifiedPeer == nil {
		only("verifyPeer %v unverifiedPeer == nil", t.dialInfo)
		return true
	}
	only("verifyPeer %v 111", t.dialInfo)

	if !unverifiedPeer.Ready() {
		only("verifyPeer %v !ready", t.dialInfo)
		return true
	} else if !unverifiedPeer.Dialable() {
		only("verifyPeer %v !dialable", t.dialInfo)
		return false
	}

	transport, exists := t.authProto.transports[unverifiedPeer.DialInfo().TransportName]
	if !exists {
		// Unsupported transport
		only("verifyPeer %v transport !exists", t.dialInfo)
		return false
	}
	only("verifyPeer %v 222", t.dialInfo)

	peerConn, err := transport.NewPeerConn(ctx, unverifiedPeer.DialInfo().DialAddr)
	if errors.Cause(err) == swarm.ErrPeerIsSelf {
		only("verifyPeer %v peer is self", t.dialInfo)
		return false
	} else if errors.Cause(err) == errors.ErrConnection {
		only("verifyPeer %v ErrConnection %+v", t.dialInfo, err)
		return true
	} else if err != nil {
		only("verifyPeer %v err %+v", t.dialInfo, err)
		return true
	}
	only("verifyPeer %v 333", t.dialInfo)

	authPeerConn, is := peerConn.(AuthPeerConn)
	if !is {
		only("verifyPeer %v ! auth peer", t.dialInfo)
		return false
	}
	defer authPeerConn.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err = authPeerConn.EnsureConnected(ctx)
	if err != nil {
		only("error connecting to peerConn (%v): %v", authPeerConn.DialInfo(), err)
		return true
	}

	err = t.authProto.ChallengePeerIdentity(ctx, authPeerConn)
	if errors.Cause(err) == errors.ErrConnection {
		// no-op
		only("verifyPeer %v err connection 2 %v", t.dialInfo, err)
		return true
	} else if errors.Cause(err) == context.Canceled {
		// no-op
		only("verifyPeer %v ctx canceled %v", t.dialInfo, err)
		return true
	} else if err != nil {
		only("error verifying peerConn identity (%v): %v", authPeerConn.DialInfo(), err)
		return true
	}
	t.authProto.Successf("authenticated with %v (addresses=%v)", t.dialInfo, authPeerConn.Addresses())
	return false
}
