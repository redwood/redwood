package redwood

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
)

type Host interface {
	ctx.Logger
	Ctx() *ctx.Context
	Start() error

	Subscribe(ctx context.Context, url string) error
	SendTx(ctx context.Context, tx Tx) error
	AddPeer(ctx context.Context, multiaddrString string) error
	Port() uint
	Transport() Transport
	Controller() Controller
	Address() Address
}

type host struct {
	*ctx.Context

	port              uint
	transport         Transport
	controller        Controller
	signingKeypair    *SigningKeypair
	encryptingKeypair *EncryptingKeypair

	subscriptionsOut map[string]subscriptionOut
	peerSeenTxs      map[string]map[Hash]bool

	peerStore map[Address]storedPeer
}

type storedPeer struct {
	id        string
	sigpubkey SigningPublicKey
	encpubkey EncryptingPublicKey
}

var (
	ErrUnsignedTx = errors.New("unsigned tx")
	ErrProtocol   = errors.New("protocol error")
	ErrPeerIsSelf = errors.New("peer is self")
)

func NewHost(signingKeypair *SigningKeypair, encryptingKeypair *EncryptingKeypair, port uint, controller Controller) (Host, error) {
	h := &host{
		Context:           &ctx.Context{},
		port:              port,
		controller:        controller,
		signingKeypair:    signingKeypair,
		encryptingKeypair: encryptingKeypair,
		subscriptionsOut:  make(map[string]subscriptionOut),
		peerSeenTxs:       make(map[string]map[Hash]bool),
		peerStore:         make(map[Address]storedPeer),
	}
	return h, nil
}

func (h *host) Ctx() *ctx.Context {
	return h.Context
}

func (h *host) Start() error {
	return h.CtxStart(
		// on startup
		func() error {
			h.SetLogLabel(h.Address().Pretty() + " host")

			// transport, err := NewLibp2pTransport(h.Address(), h.port)
			transport, err := NewHTTPTransport(h.Address(), h.port, h.controller)
			if err != nil {
				return err
			}

			transport.SetTxHandler(h.onTxReceived)
			transport.SetPrivateTxHandler(h.onPrivateTxReceived)
			transport.SetAckHandler(h.onAckReceived)
			transport.SetVerifyAddressHandler(h.onVerifyAddressReceived)
			h.transport = transport

			h.CtxAddChild(h.transport.Ctx(), nil)
			h.CtxAddChild(h.controller.Ctx(), nil)

			err = h.controller.Start()
			if err != nil {
				return err
			}
			return h.transport.Start()
		},
		nil,
		nil,
		// on shutdown
		func() {},
	)
}

func (h *host) Port() uint {
	return h.port
}

func (h *host) Transport() Transport {
	return h.transport
}

func (h *host) Controller() Controller {
	return h.controller
}

func (h *host) Address() Address {
	return h.signingKeypair.Address()
}

func (h *host) onTxReceived(tx Tx, peer Peer) {
	h.Infof(0, "tx %v received", tx.Hash().Pretty())
	h.markTxSeenByPeer(peer.ID(), tx.Hash())

	if !h.controller.HaveTx(tx.Hash()) {
		err := h.controller.AddTx(&tx)
		if err != nil {
			h.Errorf("error adding tx to controller: %v", err)
		}

		err = h.broadcastTx(context.TODO(), tx)
		if err != nil {
			h.Errorf("error rebroadcasting tx: %v", err)
		}
	}

	err := peer.WriteMsg(Msg{Type: MsgType_Ack, Payload: tx.Hash()})
	if err != nil {
		h.Errorf("error ACKing peer: %v", err)
	}
}

func (h *host) onPrivateTxReceived(encryptedTx EncryptedTx, peer Peer) {
	h.Infof(0, "private tx %v received", encryptedTx.TxHash.Pretty())
	h.markTxSeenByPeer(peer.ID(), encryptedTx.TxHash)

	bs, err := h.encryptingKeypair.OpenMessageFrom(EncryptingPublicKeyFromBytes(encryptedTx.SenderPublicKey), encryptedTx.EncryptedPayload)
	if err != nil {
		h.Errorf("error decrypting tx: %v", err)
		return
	}

	var tx Tx
	err = json.Unmarshal(bs, &tx)
	if err != nil {
		h.Errorf("error decoding tx: %v", err)
		return
	}

	if encryptedTx.TxHash != tx.Hash() {
		h.Errorf("private tx hash does not match")
		return
	}

	if !h.controller.HaveTx(tx.Hash()) {
		// Add to controller
		err := h.controller.AddTx(&tx)
		if err != nil {
			h.Errorf("error adding tx to controller: %v", err)
		}

		// Broadcast to subscribed peers
		err = h.broadcastTx(context.TODO(), tx)
		if err != nil {
			h.Errorf("error rebroadcasting tx: %v", err)
		}
	}

	err = peer.WriteMsg(Msg{Type: MsgType_Ack, Payload: tx.Hash()})
	if err != nil {
		h.Errorf("error ACKing peer: %v", err)
	}
}

func (h *host) onAckReceived(txHash Hash, peer Peer) {
	h.Infof(0, "ack received for %v from %v", txHash, peer.ID())
	h.markTxSeenByPeer(peer.ID(), txHash)
}

func (h *host) markTxSeenByPeer(peerID string, txHash Hash) {
	if h.peerSeenTxs[peerID] == nil {
		h.peerSeenTxs[peerID] = make(map[Hash]bool)
	}
	h.peerSeenTxs[peerID][txHash] = true
}

func (h *host) AddPeer(ctx context.Context, multiaddrString string) error {
	peer, err := h.transport.GetPeer(ctx, multiaddrString)
	if err != nil {
		return err
	}

	sigpubkey, _, err := h.requestPeerCredentials(ctx, peer)
	if err != nil {
		return err
	}

	h.Infof(0, "added peer with address %v", sigpubkey.Address())
	return nil
}

func (h *host) Subscribe(ctx context.Context, url string) error {
	_, exists := h.subscriptionsOut[url]
	if exists {
		return errors.New("already subscribed to " + url)
	}

	var peer Peer

	// @@TODO: subscribe to more than one peer?
	err := h.transport.ForEachProviderOfURL(ctx, url, func(p Peer) (bool, error) {
		err := p.EnsureConnected(ctx)
		if err != nil {
			return true, err
		}
		peer = p
		return false, nil
	})
	if err != nil {
		return errors.WithStack(err)
	} else if peer == nil {
		return errors.WithStack(ErrNoPeersForURL)
	}

	err = peer.WriteMsg(Msg{Type: MsgType_Subscribe, Payload: url})
	if err != nil {
		return errors.WithStack(err)
	}

	chDone := make(chan struct{})
	h.subscriptionsOut[url] = subscriptionOut{peer, chDone}

	go func() {
		defer peer.CloseConn()
		for {
			select {
			case <-chDone:
				return
			default:
			}

			msg, err := peer.ReadMsg()
			if err != nil {
				h.Errorf("error reading: %v", err)
				return
			}

			if msg.Type != MsgType_Put {
				panic("protocol error")
			}

			tx := msg.Payload.(Tx)
			tx.URL = url
			h.onTxReceived(tx, peer)

			// @@TODO: ACK the PUT
		}
	}()

	return nil
}

func (h *host) requestPeerCredentials(ctx context.Context, peer Peer) (SigningPublicKey, EncryptingPublicKey, error) {
	err := peer.EnsureConnected(ctx)
	if err != nil {
		return nil, nil, err
	}

	challengeMsg, err := GenerateChallengeMsg()
	if err != nil {
		return nil, nil, err
	}

	err = peer.WriteMsg(Msg{Type: MsgType_VerifyAddress, Payload: challengeMsg})
	if err != nil {
		return nil, nil, err
	}

	msg, err := peer.ReadMsg()
	if err != nil {
		return nil, nil, err
	} else if msg.Type != MsgType_VerifyAddressResponse {
		return nil, nil, errors.WithStack(ErrProtocol)
	}

	resp, ok := msg.Payload.(VerifyAddressResponse)
	if !ok {
		return nil, nil, errors.WithStack(ErrProtocol)
	}

	sigpubkey, err := RecoverSigningPubkey(HashBytes(challengeMsg), resp.Signature)
	if err != nil {
		return nil, nil, err
	}

	encpubkey := EncryptingPublicKeyFromBytes(resp.EncryptingPublicKey)

	h.peerStore[sigpubkey.Address()] = storedPeer{peer.ID(), sigpubkey, encpubkey}

	return sigpubkey, encpubkey, nil
}

func (h *host) onVerifyAddressReceived(challengeMsg []byte, peer Peer) error {
	defer peer.CloseConn()

	sig, err := h.signingKeypair.SignHash(HashBytes(challengeMsg))
	if err != nil {
		return err
	}

	return peer.WriteMsg(Msg{Type: MsgType_VerifyAddressResponse, Payload: VerifyAddressResponse{
		Signature:           sig,
		EncryptingPublicKey: h.encryptingKeypair.EncryptingPublicKey.Bytes(),
	}})
}

func (h *host) peerWithAddress(ctx context.Context, address Address) (Peer, EncryptingPublicKey, error) {
	if address == h.Address() {
		return nil, nil, ErrPeerIsSelf
	}

	if storedPeer, exists := h.peerStore[address]; exists {
		peer, err := h.transport.GetPeer(ctx, storedPeer.id)
		if err != nil {
			return nil, nil, err
		}
		return peer, storedPeer.encpubkey, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	chPeers, err := h.transport.PeersClaimingAddress(ctx, address)
	if err != nil {
		return nil, nil, err
	}

	for peer := range chPeers {
		err = peer.EnsureConnected(context.TODO())
		if err != nil {
			return nil, nil, err
		}
		defer peer.CloseConn()

		signingPubkey, encryptingPubkey, err := h.requestPeerCredentials(ctx, peer)
		if err != nil {
			continue
		} else if signingPubkey.Address() != address {
			return nil, nil, ErrInvalidSignature
		}

		return peer, encryptingPubkey, nil
	}
	return nil, nil, nil
}

func (h *host) broadcastTx(ctx context.Context, tx Tx) error {
	// @@TODO: should we also send all PUTs to some set of authoritative peers (like a central server)?

	if len(tx.Sig) == 0 {
		return ErrUnsignedTx
	}

	if len(tx.Recipients) > 0 {
		marshalledTx, err := json.Marshal(tx)
		if err != nil {
			return err
		}

		for _, recipientAddr := range tx.Recipients {
			if recipientAddr == h.Address() {
				continue
			}

			err := func() error {
				peer, encryptingPubkey, err := h.peerWithAddress(ctx, recipientAddr)
				if err != nil {
					return err
				} else if peer == nil {
					h.Errorf("couldn't find peer with address %s", recipientAddr)
					return nil
				}

				err = peer.EnsureConnected(context.TODO())
				if err != nil {
					return err
				}
				defer peer.CloseConn()

				msgEncrypted, err := h.encryptingKeypair.SealMessageFor(encryptingPubkey, marshalledTx)
				if err != nil {
					return err
				}

				err = peer.WriteMsg(Msg{
					Type: MsgType_Private,
					Payload: EncryptedTx{
						TxHash:           tx.Hash(),
						EncryptedPayload: msgEncrypted,
						SenderPublicKey:  h.encryptingKeypair.EncryptingPublicKey.Bytes(),
					},
				})
				if err != nil {
					return err
				}
				return nil
			}()
			if err != nil {
				return err
			}

			// @@TODO: wait for ack?
		}

	} else {
		// @@TODO: do we need to trim the tx's patches' keypaths so that they don't include
		// the keypath that the subscription is listening to?

		err := h.transport.ForEachSubscriberToURL(ctx, tx.URL, func(peer Peer) (bool, error) {
			if h.peerSeenTxs[peer.ID()][tx.Hash()] {
				return true, nil
			}

			err := peer.EnsureConnected(context.TODO())
			if err != nil {
				// @@TODO: just log, don't break?
				return true, errors.WithStack(err)
			}

			err = peer.WriteMsg(Msg{Type: MsgType_Put, Payload: tx})
			if err != nil {
				// @@TODO: just log, don't break?
				return true, errors.WithStack(err)
			}
			return true, nil
		})
		return err
	}
	return nil
}

func (h *host) SendTx(ctx context.Context, tx Tx) error {
	h.Info(0, "adding tx ", tx.Hash().Pretty())

	if len(tx.Sig) == 0 {
		err := h.SignTx(&tx)
		if err != nil {
			return err
		}
	}

	err := h.controller.AddTx(&tx)
	if err != nil {
		return err
	}

	err = h.broadcastTx(h.Ctx(), tx)
	if err != nil {
		return err
	}

	return nil
}

func (h *host) SignTx(tx *Tx) error {
	var err error
	tx.Sig, err = h.signingKeypair.SignHash(tx.Hash())
	return err
}
