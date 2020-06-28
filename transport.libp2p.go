package redwood

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	cid "github.com/ipfs/go-cid"
	dstore "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	libp2p "github.com/libp2p/go-libp2p"
	cryptop2p "github.com/libp2p/go-libp2p-crypto"
	p2phost "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	metrics "github.com/libp2p/go-libp2p-metrics"
	netp2p "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	ma "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type libp2pTransport struct {
	*ctx.Context

	libp2pHost p2phost.Host
	dht        *dht.IpfsDHT
	peerID     peer.ID
	port       uint
	p2pKey     cryptop2p.PrivKey
	*metrics.BandwidthCounter

	address types.Address
	enckeys *EncryptingKeypair

	host          Host
	controllerHub ControllerHub
	refStore      RefStore
	peerStore     PeerStore
}

const (
	PROTO_MAIN protocol.ID = "/redwood/main/1.0.0"
)

func NewLibp2pTransport(
	addr types.Address,
	port uint,
	keyfilePath string,
	enckeys *EncryptingKeypair,
	controllerHub ControllerHub,
	refStore RefStore,
	peerStore PeerStore,
) (Transport, error) {
	p2pKey, err := obtainP2PKey(keyfilePath)
	if err != nil {
		return nil, err
	}

	t := &libp2pTransport{
		Context:       &ctx.Context{},
		port:          port,
		address:       addr,
		enckeys:       enckeys,
		p2pKey:        p2pKey,
		controllerHub: controllerHub,
		refStore:      refStore,
		peerStore:     peerStore,
	}
	return t, nil
}

func (t *libp2pTransport) Start() error {
	return t.CtxStart(
		// on startup
		func() error {
			t.SetLogLabel("libp2p")
			t.Infof(0, "opening libp2p on port %v", t.port)

			peerID, err := peer.IDFromPublicKey(t.p2pKey.GetPublic())
			if err != nil {
				return err
			}
			t.peerID = peerID

			t.BandwidthCounter = metrics.NewBandwidthCounter()

			// Initialize the libp2p host
			libp2pHost, err := libp2p.New(t.Ctx(),
				libp2p.ListenAddrStrings(
					fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", t.port),
				),
				libp2p.Identity(t.p2pKey),
				libp2p.BandwidthReporter(t.BandwidthCounter),
				libp2p.NATPortMap(),
			)
			if err != nil {
				return errors.Wrap(err, "could not initialize libp2p host")
			}
			t.libp2pHost = libp2pHost

			// Initialize the DHT
			t.dht = dht.NewDHT(t.Ctx(), t.libp2pHost, dsync.MutexWrap(dstore.NewMapDatastore()))
			t.dht.Validator = blankValidator{} // Set a pass-through validator

			t.libp2pHost.SetStreamHandler(PROTO_MAIN, t.handleIncomingStream)
			t.libp2pHost.Network().Notify(t)

			go t.periodicallyAnnounceContent()
			go t.periodicallyUpdatePeerStore()

			t.Infof(0, "libp2p peer ID is %v", t.Libp2pPeerID())

			return nil
		},
		nil,
		nil,
		// on shutdown
		nil,
	)
}

func (t *libp2pTransport) Name() string {
	return "libp2p"
}

func (t *libp2pTransport) Libp2pPeerID() string {
	return t.libp2pHost.ID().Pretty()
}

func (t *libp2pTransport) ListenAddrs() []string {
	addrs := []string{}
	for _, addr := range t.libp2pHost.Addrs() {
		addrs = append(addrs, addr.String()+"/p2p/"+t.libp2pHost.ID().Pretty())
	}
	return addrs
}

func (t *libp2pTransport) Peers() []peerstore.PeerInfo {
	return peerstore.PeerInfos(t.libp2pHost.Peerstore(), t.libp2pHost.Peerstore().Peers())
}

func (t *libp2pTransport) SetHost(h Host) {
	t.host = h
}

func (t *libp2pTransport) handleIncomingStream(stream netp2p.Stream) {
	msg, err := libp2pReadMsg(stream)
	if err != nil {
		t.Errorf("incoming stream error: %v", err)
		stream.Close()
		return
	}

	peer := t.makeConnectedPeer(stream)

	switch msg.Type {
	case MsgType_Subscribe:
		stateURI, ok := msg.Payload.(string)
		if !ok {
			t.Errorf("Subscribe message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		fromTxID := types.ID{}
		toTxID := types.ID{}

		err := t.host.HandleFetchHistoryRequest(stateURI, fromTxID, toTxID, peer)
		if err != nil {
			t.Errorf("error fetching history: %v", err)
			peer.CloseConn()
			return
		}

		t.host.HandleIncomingTxSubscription(stateURI, peer)

	case MsgType_Put:
		defer peer.CloseConn()

		tx, ok := msg.Payload.(Tx)
		if !ok {
			t.Errorf("Put message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandleTxReceived(tx, peer)

	case MsgType_Ack:
		defer peer.CloseConn()

		ackMsg, ok := msg.Payload.(libp2pAckMsg)
		if !ok {
			t.Errorf("Ack message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandleAckReceived(ackMsg.StateURI, ackMsg.TxID, peer)

	case MsgType_ChallengeIdentityRequest:
		defer peer.CloseConn()

		challengeMsg, ok := msg.Payload.(types.ChallengeMsg)
		if !ok {
			t.Errorf("MsgType_ChallengeIdentityRequest message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		err := t.host.HandleChallengeIdentity(challengeMsg, peer)
		if err != nil {
			t.Errorf("MsgType_ChallengeIdentityRequest: error from verifyAddressHandler: %v", err)
			return
		}

	case MsgType_FetchRef:
		defer peer.CloseConn()

		refID, ok := msg.Payload.(types.RefID)
		if !ok {
			t.Errorf("FetchRef message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandleFetchRefReceived(refID, peer)

	case MsgType_Private:
		defer peer.CloseConn()

		encryptedTx, ok := msg.Payload.(EncryptedTx)
		if !ok {
			t.Errorf("Private message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		bs, err := t.enckeys.OpenMessageFrom(
			EncryptingPublicKeyFromBytes(encryptedTx.SenderPublicKey),
			encryptedTx.EncryptedPayload,
		)
		if err != nil {
			t.Errorf("error decrypting tx: %v", err)
			return
		}

		var tx Tx
		err = json.Unmarshal(bs, &tx)
		if err != nil {
			t.Errorf("error decoding tx: %v", err)
			return
		} else if encryptedTx.TxID != tx.ID {
			t.Errorf("private tx id does not match")
			return
		}
		t.host.HandleTxReceived(tx, peer)

	case MsgType_AdvertisePeers:
		defer peer.CloseConn()

		tuples, ok := msg.Payload.([]PeerDialInfo)
		if !ok {
			t.Errorf("Advertise peers: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.peerStore.AddDialInfos(tuples)

	default:
		panic("protocol error")
	}
}

func (t *libp2pTransport) NewPeerConn(ctx context.Context, dialAddr string) (Peer, error) {
	addr, err := ma.NewMultiaddr(dialAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse multiaddr '%v'", dialAddr)
	}
	pinfo, err := peerstore.InfoFromP2pAddr(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse PeerInfo from multiaddr '%v'", dialAddr)
	} else if pinfo.ID == t.peerID {
		return nil, errors.WithStack(ErrPeerIsSelf)
	}
	peer := t.makeDisconnectedPeer(*pinfo)
	return peer, nil
}

func (t *libp2pTransport) ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan Peer, error) {
	urlCid, err := cidForString("serve:" + stateURI)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ch := make(chan Peer)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			for pinfo := range t.dht.FindProvidersAsync(ctx, urlCid, 8) {
				if pinfo.ID == t.libp2pHost.ID() {
					select {
					case <-ctx.Done():
						return
					default:
						continue
					}
				}

				// @@TODO: validate peer as an authorized provider via web of trust, certificate authority,
				// whitelist, etc.

				peer := t.makeDisconnectedPeer(pinfo)
				if peer == nil || peer.DialInfo().DialAddr == "" {
					continue
				}

				select {
				case ch <- peer:
				case <-ctx.Done():
					return
				}
			}
			time.Sleep(1 * time.Second) // @@TODO: make configurable?
		}
	}()
	return ch, nil
}

func (t *libp2pTransport) ProvidersOfRef(ctx context.Context, refID types.RefID) (<-chan Peer, error) {
	refCid, err := cidForString("ref:" + refID.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ch := make(chan Peer)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			for pinfo := range t.dht.FindProvidersAsync(ctx, refCid, 8) {
				if pinfo.ID == t.libp2pHost.ID() {
					continue
				}

				t.Infof(0, `found peer %v for ref "%v"`, pinfo.ID, refID.String())

				peer := t.makeDisconnectedPeer(pinfo)
				if peer == nil || peer.DialInfo().DialAddr == "" {
					continue
				}

				select {
				case ch <- peer:
				case <-ctx.Done():
				}
			}
		}
	}()
	return ch, nil
}

func (t *libp2pTransport) PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan Peer, error) {
	addrCid, err := cidForString("addr:" + address.String())
	if err != nil {
		t.Errorf("announce: error creating cid: %v", err)
		return nil, err
	}

	ch := make(chan Peer)

	go func() {
		defer close(ch)

		for pinfo := range t.dht.FindProvidersAsync(ctx, addrCid, 8) {
			if pinfo.ID == t.libp2pHost.ID() {
				continue
			}

			peer := t.makeDisconnectedPeer(pinfo)
			if peer == nil || peer.DialInfo().DialAddr == "" {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case <-t.Ctx().Done():
				return
			case ch <- peer:
			}
		}
	}()

	return ch, nil
}

// Periodically announces our repos and objects to the network.
func (t *libp2pTransport) periodicallyAnnounceContent() {
	for {
		select {
		case <-t.Ctx().Done():
			return
		default:
		}

		// t.Info(0, "announce")

		// Announce the URLs we're serving
		stateURIs, err := t.controllerHub.KnownStateURIs()
		if err != nil {
			t.Errorf("error fetching known state URIs from DB: %v", err)

		} else {
			for _, stateURI := range stateURIs {
				stateURI := stateURI
				go func() {
					ctxInner, cancel := context.WithTimeout(t.Ctx(), 10*time.Second)
					defer cancel()

					c, err := cidForString("serve:" + stateURI)
					if err != nil {
						t.Errorf("announce: error creating cid: %v", err)
						return
					}

					err = t.dht.Provide(ctxInner, c, true)
					if err != nil && err != kbucket.ErrLookupFailure {
						t.Errorf(`announce: could not dht.Provide stateURI "%v": %v`, stateURI, err)
						return
					}
				}()
			}
		}

		// Announce the blobs we're serving
		refHashes, err := t.refStore.AllHashes()
		if err != nil {
			t.Errorf("error fetching refStore hashes: %v", err)
		}
		for _, refHash := range refHashes {
			refHash := refHash
			go func() {
				ctx, cancel := context.WithTimeout(t.Ctx(), 10*time.Second)
				defer cancel()

				err := t.AnnounceRef(ctx, refHash)
				if err != nil {
					t.Errorf("announce: error: %v", err)
				}
			}()
		}

		// Advertise our address (for exchanging private txs)
		go func() {
			ctxInner, cancel := context.WithTimeout(t.Ctx(), 10*time.Second)
			defer cancel()

			c, err := cidForString("addr:" + t.address.String())
			if err != nil {
				t.Errorf("announce: error creating cid: %v", err)
				return
			}

			err = t.dht.Provide(ctxInner, c, true)
			if err != nil && err != kbucket.ErrLookupFailure {
				t.Errorf(`announce: could not dht.Provide pubkey: %v`, err)
			}
		}()

		time.Sleep(10 * time.Second)
	}
}

func (t *libp2pTransport) periodicallyUpdatePeerStore() {
	for {
		// Update our node's info in the peer store
		myAddrs := NewStringSet(nil)
		for _, addr := range t.libp2pHost.Addrs() {
			addrStr := addr.String()
			myAddrs.Add(addrStr)
		}
		myAddrs = cleanLibp2pAddrs(myAddrs, t.peerID)

		if len(myAddrs) > 0 {
			host := t.host.(*host)
			for addr := range myAddrs {
				t.peerStore.AddVerifiedCredentials(
					PeerDialInfo{TransportName: t.Name(), DialAddr: addr},
					host.signingKeypair.SigningPublicKey.Address(),
					host.signingKeypair.SigningPublicKey,
					host.encryptingKeypair.EncryptingPublicKey,
				)
			}
		}

		// Ensure all peers discovered on the libp2p layer are in the peer store
		for _, pinfo := range t.Peers() {
			if pinfo.ID == peer.ID("") {
				continue
			}
			addrs := multiaddrsFromPeerInfo(pinfo)
			if len(addrs) > 0 {
				var dialInfos []PeerDialInfo
				for addr := range addrs {
					dialInfos = append(dialInfos, PeerDialInfo{TransportName: t.Name(), DialAddr: addr})
				}
				t.peerStore.AddDialInfos(dialInfos)
			}
		}

		// Share our peer store with all of our peers
		peerDialInfos := t.peerStore.AllDialInfos()
		for _, pinfo := range t.Peers() {
			if pinfo.ID == t.libp2pHost.ID() {
				continue
			}
			func() {
				peer := t.makeDisconnectedPeer(pinfo)
				if peer == nil {
					return
				}

				err := peer.EnsureConnected(context.TODO())
				if err != nil {
					// t.Errorf("error connecting to peer: %+v", err)
					return
				}
				defer peer.CloseConn()

				err = peer.writeMsg(Msg{Type: MsgType_AdvertisePeers, Payload: peerDialInfos})
				if err != nil {
					// t.Errorf("error writing to peer: %+v", err)
				}
			}()
		}

		time.Sleep(10 * time.Second) // @@TODO: make configurable
	}
}

func (t *libp2pTransport) AnnounceRef(ctx context.Context, refID types.RefID) error {
	c, err := cidForString("ref:" + refID.String())
	if err != nil {
		return err
	}

	err = t.dht.Provide(ctx, c, true)
	if err != nil && err != kbucket.ErrLookupFailure {
		t.Errorf(`announce: could not dht.Provide ref "%v": %v`, refID.String(), err)
		return err
	}

	return nil
}

func (t *libp2pTransport) makeConnectedPeer(stream netp2p.Stream) *libp2pPeer {
	pinfo := t.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())
	peer := &libp2pPeer{t: t, pinfo: pinfo, stream: stream}
	dialAddrs := multiaddrsFromPeerInfo(pinfo)

	var dialInfos []PeerDialInfo
	for dialAddr := range dialAddrs {
		dialInfo := PeerDialInfo{TransportName: t.Name(), DialAddr: dialAddr}
		dialInfos = append(dialInfos, dialInfo)
	}
	t.peerStore.AddDialInfos(dialInfos)

	for _, dialInfo := range dialInfos {
		peer.PeerDetails = t.peerStore.PeerWithDialInfo(dialInfo)
		if peer.PeerDetails != nil {
			break
		}
	}
	return peer
}

func (t *libp2pTransport) makeDisconnectedPeer(pinfo peerstore.PeerInfo) *libp2pPeer {
	peer := &libp2pPeer{t: t, pinfo: pinfo, stream: nil}
	dialAddrs := multiaddrsFromPeerInfo(pinfo)
	if len(dialAddrs) == 0 {
		return nil
	}

	var peerDetails PeerDetails
	for dialAddr := range dialAddrs {
		peerDetails = t.peerStore.PeerWithDialInfo(PeerDialInfo{TransportName: t.Name(), DialAddr: dialAddr})
		if peerDetails != nil {
			break
		}
	}
	if peerDetails != nil {
		peer.PeerDetails = peerDetails
	} else {
		var dialInfos []PeerDialInfo
		for dialAddr := range dialAddrs {
			dialInfos = append(dialInfos, PeerDialInfo{TransportName: t.Name(), DialAddr: dialAddr})
		}
		t.peerStore.AddDialInfos(dialInfos)
		for _, dialInfo := range dialInfos {
			peer.PeerDetails = t.peerStore.PeerWithDialInfo(dialInfo)
			if peer.PeerDetails != nil {
				break
			}
		}
	}
	return peer
}

type libp2pPeer struct {
	PeerDetails
	t      *libp2pTransport
	pinfo  peerstore.PeerInfo
	stream netp2p.Stream
}

func (peer *libp2pPeer) Transport() Transport {
	return peer.t
}

func (peer *libp2pPeer) ReachableAt() StringSet {
	return multiaddrsFromPeerInfo(peer.pinfo)
}

func (peer *libp2pPeer) EnsureConnected(ctx context.Context) error {
	if peer.stream == nil {
		if len(peer.t.libp2pHost.Network().ConnsToPeer(peer.pinfo.ID)) == 0 {
			err := peer.t.libp2pHost.Connect(ctx, peer.pinfo)
			if err != nil {
				return errors.Wrapf(types.ErrConnection, "(peer %v): %v", peer.pinfo.ID, err)
			}
		}

		stream, err := peer.t.libp2pHost.NewStream(ctx, peer.pinfo.ID, PROTO_MAIN)
		if err != nil {
			peer.UpdateConnStats(false)
			return err
		}
		peer.UpdateConnStats(true)

		peer.stream = stream
	}
	return nil
}

func (peer *libp2pPeer) Subscribe(ctx context.Context, stateURI string) (TxSubscriptionClient, error) {
	err := peer.EnsureConnected(ctx)
	if err != nil {
		peer.t.Errorf("error connecting to peer: %v", err)
		return nil, err
	}

	err = peer.writeMsg(Msg{Type: MsgType_Subscribe, Payload: stateURI})
	if err != nil {
		return nil, err
	}

	return &libp2pTxSubscriptionClient{
		stateURI: stateURI,
		peer:     peer,
	}, nil
}

type libp2pTxSubscriptionClient struct {
	peer     *libp2pPeer
	stateURI string
}

func (sub *libp2pTxSubscriptionClient) Read() (*Tx, []types.ID, error) {
	msg, err := sub.peer.readMsg()
	if err != nil {
		return nil, nil, errors.Errorf("error reading from subscription: %v", err)
	}

	switch msg.Type {
	case MsgType_Put:
		tx := msg.Payload.(Tx)
		return &tx, nil, nil

	case MsgType_Private:
		encryptedTx, ok := msg.Payload.(EncryptedTx)
		if !ok {
			return nil, nil, errors.Errorf("Private message: bad payload: (%T) %v", msg.Payload, msg.Payload)
		}
		bs, err := sub.peer.t.enckeys.OpenMessageFrom(
			EncryptingPublicKeyFromBytes(encryptedTx.SenderPublicKey),
			encryptedTx.EncryptedPayload,
		)
		if err != nil {
			return nil, nil, errors.Errorf("error decrypting tx: %v", err)
		}

		var tx Tx
		err = json.Unmarshal(bs, &tx)
		if err != nil {
			return nil, nil, errors.Errorf("error decoding tx: %v", err)
		} else if encryptedTx.TxID != tx.ID {
			return nil, nil, errors.Errorf("private tx id does not match")
		}
	}
	return nil, nil, errors.New("protocol error, expecting MsgType_Put or MsgType_Private")
}

func (sub *libp2pTxSubscriptionClient) Close() error {
	return sub.peer.CloseConn()
}

func (p *libp2pPeer) Put(tx Tx, leaves []types.ID) error {
	return p.writeMsg(Msg{Type: MsgType_Put, Payload: tx})
}

func (peer *libp2pPeer) PutPrivate(tx Tx, leaves []types.ID) error {
	marshalledTx, err := json.Marshal(tx)
	if err != nil {
		return errors.WithStack(err)
	}

	_, peerEncPubkey := peer.PublicKeypairs()

	encryptedTxBytes, err := peer.t.enckeys.SealMessageFor(peerEncPubkey, marshalledTx)
	if err != nil {
		return errors.WithStack(err)
	}

	etx := EncryptedTx{
		TxID:             tx.ID,
		EncryptedPayload: encryptedTxBytes,
		SenderPublicKey:  peer.t.enckeys.EncryptingPublicKey.Bytes(),
	}

	return peer.writeMsg(Msg{Type: MsgType_Private, Payload: etx})
}

func (p *libp2pPeer) PutState(state tree.Node, leaves []types.ID) error {
	panic("unimplemented")
	return nil
}

type libp2pAckMsg struct {
	StateURI string   `json:"stateURI"`
	TxID     types.ID `json:"txID"`
}

func (p *libp2pPeer) Ack(stateURI string, txID types.ID) error {
	return p.writeMsg(Msg{Type: MsgType_Ack, Payload: libp2pAckMsg{stateURI, txID}})
}

func (p *libp2pPeer) ChallengeIdentity(challengeMsg types.ChallengeMsg) error {
	return p.writeMsg(Msg{Type: MsgType_ChallengeIdentityRequest, Payload: challengeMsg})
}

func (p *libp2pPeer) ReceiveChallengeIdentityResponse() (ChallengeIdentityResponse, error) {
	msg, err := p.readMsg()
	if err != nil {
		return ChallengeIdentityResponse{}, err
	}

	resp, ok := msg.Payload.(ChallengeIdentityResponse)
	if !ok {
		return ChallengeIdentityResponse{}, ErrProtocol
	}

	return resp, nil
}

func (p *libp2pPeer) RespondChallengeIdentity(challengeIdentityResponse ChallengeIdentityResponse) error {
	return p.writeMsg(Msg{Type: MsgType_ChallengeIdentityResponse, Payload: challengeIdentityResponse})
}

func (p *libp2pPeer) FetchRef(refID types.RefID) error {
	return p.writeMsg(Msg{Type: MsgType_FetchRef, Payload: refID})
}

func (p *libp2pPeer) SendRefHeader() error {
	return p.writeMsg(Msg{Type: MsgType_FetchRefResponse, Payload: FetchRefResponse{Header: &FetchRefResponseHeader{}}})
}

func (p *libp2pPeer) SendRefPacket(data []byte, end bool) error {
	return p.writeMsg(Msg{Type: MsgType_FetchRefResponse, Payload: FetchRefResponse{Body: &FetchRefResponseBody{Data: data, End: end}}})
}

func (p *libp2pPeer) ReceiveRefPacket() (FetchRefResponseBody, error) {
	msg, err := p.readMsg()
	if err != nil {
		return FetchRefResponseBody{}, errors.Errorf("error reading from peer: %v", err)
	} else if msg.Type != MsgType_FetchRefResponse {
		return FetchRefResponseBody{}, ErrProtocol
	}

	resp, is := msg.Payload.(FetchRefResponse)
	if !is {
		return FetchRefResponseBody{}, ErrProtocol
	} else if resp.Body == nil {
		return FetchRefResponseBody{}, ErrProtocol
	}
	return *resp.Body, nil
}

func (p *libp2pPeer) ReceiveRefHeader() (FetchRefResponseHeader, error) {
	msg, err := p.readMsg()
	if err != nil {
		return FetchRefResponseHeader{}, errors.Errorf("error reading from peer: %v", err)
	} else if msg.Type != MsgType_FetchRefResponse {
		return FetchRefResponseHeader{}, ErrProtocol
	}

	resp, is := msg.Payload.(FetchRefResponse)
	if !is {
		return FetchRefResponseHeader{}, ErrProtocol
	} else if resp.Header == nil {
		return FetchRefResponseHeader{}, ErrProtocol
	}
	return *resp.Header, nil
}

func (p *libp2pPeer) writeMsg(msg Msg) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	bs, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	buflen := uint64(len(bs))

	err = WriteUint64(p.stream, buflen)
	if err != nil {
		return err
	}
	n, err := io.Copy(p.stream, bytes.NewReader(bs))
	if err != nil {
		return err
	} else if n != int64(buflen) {
		return errors.New("WriteMsg: could not write entire packet")
	}
	return nil
}

func (p *libp2pPeer) readMsg() (msg Msg, err error) {
	defer func() { p.UpdateConnStats(err == nil) }()
	return libp2pReadMsg(p.stream)
}

func libp2pReadMsg(r io.Reader) (msg Msg, err error) {
	size, err := ReadUint64(r)
	if err != nil {
		return Msg{}, err
	}

	buf := &bytes.Buffer{}
	_, err = io.CopyN(buf, r, int64(size))
	if err != nil {
		return Msg{}, err
	}

	err = json.NewDecoder(buf).Decode(&msg)
	return msg, err
}

func (p *libp2pPeer) CloseConn() error {
	if p.stream != nil {
		return p.stream.Close()
	}
	return nil
}

func obtainP2PKey(keyfilePath string) (cryptop2p.PrivKey, error) {
	f, err := os.Open(keyfilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err

	} else if err == nil {
		defer f.Close()

		data, err := ioutil.ReadFile(keyfilePath)
		if err != nil {
			return nil, err
		}
		return cryptop2p.UnmarshalPrivateKey(data)
	}

	p2pKey, _, err := cryptop2p.GenerateKeyPair(cryptop2p.Secp256k1, 0)
	if err != nil {
		return nil, err
	}

	bs, err := p2pKey.Bytes()
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(keyfilePath, bs, 0400)
	if err != nil {
		return nil, err
	}

	return p2pKey, nil
}

func cidForString(s string) (cid.Cid, error) {
	pref := cid.NewPrefixV1(cid.Raw, multihash.SHA2_256)
	c, err := pref.Sum([]byte(s))
	if err != nil {
		return cid.Cid{}, errors.Wrap(err, "could not create cid")
	}
	return c, nil
}

func multiaddrsFromPeerInfo(pinfo peerstore.PeerInfo) StringSet {
	dialAddrs := NewStringSet(nil)
	multiaddrs, err := peerstore.InfoToP2pAddrs(&pinfo)
	if err != nil {
		panic(err)
	}
	for _, addr := range multiaddrs {
		dialAddrs.Add(addr.String())
	}
	return cleanLibp2pAddrs(dialAddrs, pinfo.ID)
}

func cleanLibp2pAddrs(addrStrs StringSet, peerID peer.ID) StringSet {
	keep := NewStringSet(nil)
	for addrStr := range addrStrs {
		if strings.Index(addrStr, "/ip4/172.") == 0 {
			continue
			// } else if strings.Index(addrStr, "/ip4/0.0.0.0") == 0 {
			//  continue
			// } else if strings.Index(addrStr, "/ip4/127.0.0.1") == 0 {
			// 	continue
		} else if addrStr[:len("/p2p-circuit")] == "/p2p-circuit" {
			continue
		}

		addrStr = strings.Replace(addrStr, "/ipfs/", "/p2p/", 1)

		if !strings.Contains(addrStr, "/p2p/") {
			addrStr = addrStr + "/p2p/" + peerID.Pretty()
		}

		keep.Add(addrStr)
	}
	return keep
}

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

type Msg struct {
	Type    MsgType     `json:"type"`
	Payload interface{} `json:"payload"`
}

type MsgType string

const (
	MsgType_Subscribe                 MsgType = "subscribe"
	MsgType_Unsubscribe               MsgType = "unsubscribe"
	MsgType_Put                       MsgType = "put"
	MsgType_Private                   MsgType = "private"
	MsgType_Ack                       MsgType = "ack"
	MsgType_Error                     MsgType = "error"
	MsgType_ChallengeIdentityRequest  MsgType = "challenge identity"
	MsgType_ChallengeIdentityResponse MsgType = "challenge identity response"
	MsgType_FetchRef                  MsgType = "fetch ref"
	MsgType_FetchRefResponse          MsgType = "fetch ref response"
	MsgType_AdvertisePeers            MsgType = "advertise peers"
)

func ReadUint64(r io.Reader) (uint64, error) {
	buf := make([]byte, 8)
	_, err := io.ReadFull(r, buf)
	if err == io.EOF {
		return 0, err
	} else if err != nil {
		return 0, errors.Wrap(err, "ReadUint64")
	}
	return binary.LittleEndian.Uint64(buf), nil
}

func WriteUint64(w io.Writer, n uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, n)
	written, err := w.Write(buf)
	if err != nil {
		return err
	} else if written < 8 {
		return errors.New("WriteUint64: wrote too few bytes")
	}
	return nil
}

func (msg *Msg) UnmarshalJSON(bs []byte) error {
	var m struct {
		Type         string          `json:"type"`
		PayloadBytes json.RawMessage `json:"payload"`
	}

	err := json.Unmarshal(bs, &m)
	if err != nil {
		return err
	}

	msg.Type = MsgType(m.Type)

	switch msg.Type {
	case MsgType_Subscribe:
		url := string(m.PayloadBytes)
		msg.Payload = url[1 : len(url)-1] // remove quotes

	case MsgType_Put:
		var tx Tx
		err := json.Unmarshal(m.PayloadBytes, &tx)
		if err != nil {
			return err
		}
		msg.Payload = tx

	case MsgType_Ack:
		var payload libp2pAckMsg
		err := json.Unmarshal(m.PayloadBytes, &payload)
		if err != nil {
			return err
		}
		msg.Payload = payload

	case MsgType_Private:
		var ep EncryptedTx
		err := json.Unmarshal(m.PayloadBytes, &ep)
		if err != nil {
			return err
		}
		msg.Payload = ep

	case MsgType_ChallengeIdentityRequest:
		var challenge types.ChallengeMsg
		err := json.Unmarshal(m.PayloadBytes, &challenge)
		if err != nil {
			return err
		}
		msg.Payload = challenge

	case MsgType_ChallengeIdentityResponse:
		var resp ChallengeIdentityResponse
		err := json.Unmarshal([]byte(m.PayloadBytes), &resp)
		if err != nil {
			return err
		}

		msg.Payload = resp

	case MsgType_FetchRef:
		var refID types.RefID
		err := json.Unmarshal([]byte(m.PayloadBytes), &refID)
		if err != nil {
			return err
		}
		msg.Payload = refID

	case MsgType_FetchRefResponse:
		var resp FetchRefResponse
		err := json.Unmarshal([]byte(m.PayloadBytes), &resp)
		if err != nil {
			return err
		}
		msg.Payload = resp

	case MsgType_AdvertisePeers:
		var peerDialInfos []PeerDialInfo
		err := json.Unmarshal([]byte(m.PayloadBytes), &peerDialInfos)
		if err != nil {
			return err
		}
		msg.Payload = peerDialInfos

	default:
		return errors.New("bad msg")
	}

	return nil
}

func (t *libp2pTransport) Listen(network netp2p.Network, multiaddr ma.Multiaddr)      {}
func (t *libp2pTransport) ListenClose(network netp2p.Network, multiaddr ma.Multiaddr) {}

func (t *libp2pTransport) Connected(network netp2p.Network, conn netp2p.Conn) {
	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	t.Debugf("libp2p connected: %v", addr)
	t.peerStore.AddDialInfos([]PeerDialInfo{{TransportName: t.Name(), DialAddr: addr}})
}

func (t *libp2pTransport) Disconnected(network netp2p.Network, conn netp2p.Conn) {
	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	t.Debugf("libp2p disconnected: %v", addr)
}

func (t *libp2pTransport) OpenedStream(network netp2p.Network, stream netp2p.Stream) {}
func (t *libp2pTransport) ClosedStream(network netp2p.Network, stream netp2p.Stream) {}
