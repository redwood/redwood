package redwood

import (
	"context"
	"encoding/json"
	"math/rand"

	"github.com/pkg/errors"
	"github.com/plan-systems/plan-core/tools/ctx"
)

type Host interface {
	Subscribe(ctx context.Context, url string) error
	AddTx(ctx context.Context, tx Tx) error
	AddPeer(ctx context.Context, multiaddrString string) error
}

type host struct {
	ctx.Context

	Port              uint
	Transport         Transport
	Store             Store
	signingKeypair    *SigningKeypair
	encryptingKeypair *EncryptingKeypair

	subscriptionsOut map[string]subscriptionOut
	peerSeenTxs      map[string]map[ID]bool
}

var (
	ErrUnsignedTx = errors.New("unsigned tx")
	ErrProtocol   = errors.New("protocol error")
)

func NewHost(signingKeypair *SigningKeypair, encryptingKeypair *EncryptingKeypair, port uint, store Store) (*host, error) {
	h := &host{
		Port:              port,
		Store:             store,
		signingKeypair:    signingKeypair,
		encryptingKeypair: encryptingKeypair,
		subscriptionsOut:  make(map[string]subscriptionOut),
		peerSeenTxs:       make(map[string]map[ID]bool),
	}

	err := h.CtxStart(
		h.ctxStartup,
		nil,
		nil,
		h.ctxStopping,
	)

	return h, err
}

func (h *host) Address() Address {
	return h.signingKeypair.Address()
}

func (h *host) ctxStartup() error {
	h.SetLogLabel(h.Address().Pretty() + " host")

	h.Infof(0, "opening libp2p on port %v", h.Port)
	// transport, err := NewLibp2pTransport(h.Ctx, h.Address(), h.Port)
	transport, err := NewHTTPTransport(h.Ctx, h.Address(), h.Port, h.Store)
	if err != nil {
		return err
	}

	transport.SetPutHandler(h.onTxReceived)
	transport.SetAckHandler(h.onAckReceived)
	transport.SetVerifyAddressHandler(h.onVerifyAddressReceived)

	h.Transport = transport

	return nil
}

func (h *host) ctxStopping() {
	// No op since h.Ctx will cancel as this ctx completes stopping
}

func (h *host) onTxReceived(tx Tx, peer Peer) {
	h.Infof(0, "tx %v received", tx.ID.Pretty())
	h.markTxSeenByPeer(peer.ID(), tx.ID)

	// @@TODO: private txs

	if !h.Store.HaveTx(tx.ID) {
		// Add to store
		err := h.Store.AddTx(&tx)
		if err != nil {
			h.Errorf("error adding tx to store: %v", err)
		}

		// Broadcast to subscribed peers
		err = h.put(context.TODO(), tx)
		if err != nil {
			h.Errorf("error rebroadcasting tx: %v", err)
		}
	}

	err := peer.WriteMsg(Msg{Type: MsgType_Ack, Payload: tx.ID})
	if err != nil {
		h.Errorf("error ACKing peer: %v", err)
	}
}

func (h *host) onAckReceived(txID ID, peer Peer) {
	h.Infof(0, "ack received for %v from %v", txID, peer.ID())
	h.markTxSeenByPeer(peer.ID(), txID)
}

func (h *host) markTxSeenByPeer(peerID string, txID ID) {
	if h.peerSeenTxs[peerID] == nil {
		h.peerSeenTxs[peerID] = make(map[ID]bool)
	}
	h.peerSeenTxs[peerID][txID] = true
}

func (h *host) AddPeer(ctx context.Context, multiaddrString string) error {
	return h.Transport.AddPeer(ctx, multiaddrString)
}

func (h *host) Subscribe(ctx context.Context, url string) error {
	_, exists := h.subscriptionsOut[url]
	if exists {
		return errors.New("already subscribed to " + url)
	}

	var peer Peer

	// @@TODO: subscribe to more than one peer?
	err := h.Transport.ForEachProviderOfURL(ctx, url, func(p Peer) (bool, error) {
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

func (h *host) verifyPeerAddress(peer Peer, address Address) error {
	challengeMsg := make([]byte, 128)
	_, err := rand.Read(challengeMsg)
	if err != nil {
		return err
	}

	err = peer.WriteMsg(Msg{Type: MsgType_VerifyAddress, Payload: challengeMsg})
	if err != nil {
		return err
	}

	msg, err := peer.ReadMsg()
	if err != nil {
		return err
	} else if msg.Type != MsgType_VerifyAddressResponse {
		return ErrProtocol
	}

	sig, ok := msg.Payload.([]byte)
	if !ok {
		return ErrProtocol
	}

	hash := HashBytes(challengeMsg)

	pubkey, err := RecoverSigningPubkey(hash, sig)
	if err != nil {
		return err
	} else if pubkey.Address() != address {
		return ErrInvalidSignature
	}
	return nil
}

func (h *host) onVerifyAddressReceived(challengeMsg []byte) ([]byte, error) {
	hash := HashBytes(challengeMsg)
	return h.signingKeypair.SignHash(hash)
}

func (h *host) peerWithAddress(ctx context.Context, address Address) (Peer, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	chPeers, err := h.Transport.PeersWithAddress(ctx, address)
	if err != nil {
		return nil, err
	}

	for peer := range chPeers {
		err = peer.EnsureConnected(context.TODO())
		if err != nil {
			return nil, err
		}
		defer peer.CloseConn()

		err = h.verifyPeerAddress(peer, address)
		if err != nil {
			continue
		}

		return peer, nil
	}
	return nil, nil
}

func (h *host) put(ctx context.Context, tx Tx) error {
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
			err := func() error {
				peer, err := h.peerWithAddress(ctx, recipientAddr)
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

				msgEncrypted, err := h.encryptingKeypair.SealMessageFor(ENCRYPTION_PUBKEY_FOR_ADDRESS[recipientAddr], marshalledTx)
				if err != nil {
					return err
				}

				err = peer.WriteMsg(Msg{Type: MsgType_Private, Payload: msgEncrypted})
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

		err := h.Transport.ForEachSubscriberToURL(ctx, tx.URL, func(peer Peer) (bool, error) {
			if h.peerSeenTxs[peer.ID()][tx.ID] {
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

// @@TODO: remove this and build a mechanism for transports to fetch the public key associated with a given address
var ENCRYPTION_PUBKEY_FOR_ADDRESS map[Address]EncryptingPublicKey

func (h *host) AddTx(ctx context.Context, tx Tx) error {
	h.Info(0, "adding tx ", tx.ID.Pretty())

	hash, err := tx.Hash()
	if err != nil {
		return err
	}

	sig, err := h.signingKeypair.SignHash(hash)
	if err != nil {
		return err
	}

	tx.Sig = sig

	err = h.Store.AddTx(&tx)
	if err != nil {
		return err
	}

	err = h.put(h.Ctx, tx)
	if err != nil {
		return err
	}

	return nil
}
