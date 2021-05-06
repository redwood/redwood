package braidhttp

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/crypto"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type peerConn struct {
	swarm.PeerDetails

	t *transport
	sync.Mutex

	// stream
	stream struct {
		io.ReadCloser
		io.Writer
		http.Flusher
	}
}

func (p *peerConn) Transport() swarm.Transport {
	return p.t
}

func (p *peerConn) EnsureConnected(ctx context.Context) error {
	return nil
}

func (p *peerConn) Subscribe(ctx context.Context, stateURI string) (_ swarm.ReadableSubscription, err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		return nil, errors.New("peer has no DialAddr")
	}

	req, err := http.NewRequest("GET", p.DialInfo().DialAddr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("State-URI", stateURI)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	subTypeBytes, err := swarm.SubscriptionType_Txs.MarshalText()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Subscribe", string(subTypeBytes))

	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "error subscribing to peer (%v) (state URI: %v)", p.DialInfo().DialAddr, stateURI)
	} else if resp.StatusCode != 200 {
		return nil, errors.Wrapf(err, "error subscribing to peer (%v) (state URI: %v)", p.DialInfo().DialAddr, stateURI)
	}

	p.t.storeAltSvcHeaderPeers(resp.Header)

	return &httpReadableSubscription{
		client:  &client,
		peer:    p,
		stream:  resp.Body,
		private: resp.Header.Get("Private") == "true",
	}, nil
}

func (p *peerConn) Put(ctx context.Context, tx *tree.Tx, state state.Node, leaves []types.ID) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	ctx, cancel := utils.CombinedContext(ctx, 10*time.Second, p.t.chStop)
	defer cancel()

	if p.DialInfo().DialAddr == "" {
		p.t.Warn("peer has no DialAddr")
		return nil
	}

	identity, err := p.t.keyStore.IdentityWithAddress(tx.From)
	if err != nil {
		return err
	}

	var (
		peerSigPubkey crypto.SigningPublicKey
		peerEncPubkey crypto.EncryptingPublicKey
	)
	if tx.IsPrivate() {
		peerAddrs := types.OverlappingAddresses(tx.Recipients, p.Addresses())
		if len(peerAddrs) == 0 {
			return errors.New("tx not intended for this peer")
		}
		peerSigPubkey, peerEncPubkey = p.PublicKeys(peerAddrs[0])
	}

	req, err := putRequestFromTx(ctx, tx, p.DialInfo().DialAddr, identity.Encrypting, peerSigPubkey.Address(), peerEncPubkey)
	if err != nil {
		return errors.WithStack(err)
	}

	resp, err := p.t.doRequest(req)
	if err != nil {
		return errors.Wrapf(err, "error PUTting tx to peer (%v)", p.DialInfo().DialAddr)
	}
	defer resp.Body.Close()
	return nil
}

func (p *peerConn) Ack(stateURI string, txID types.ID) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		p.t.Warn("peer has no DialAddr")
		return nil
	}

	txIDBytes, err := txID.MarshalText()
	if err != nil {
		return errors.WithStack(err)
	}

	ctx, cancel := utils.CombinedContext(10*time.Second, p.t.chStop)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "ACK", p.DialInfo().DialAddr, bytes.NewReader(txIDBytes))
	if err != nil {
		return err
	}
	req.Header.Set("State-URI", stateURI)

	resp, err := p.t.doRequest(req)
	if err != nil {
		return errors.Wrapf(err, "error ACKing to peer (%v)", p.DialInfo().DialAddr)
	}
	defer resp.Body.Close()
	return nil
}

func (p *peerConn) ChallengeIdentity(challengeMsg types.ChallengeMsg) (err error) {
	defer utils.WithStack(&err)
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		p.t.Warn("peer has no DialAddr")
		return nil
	}

	ctx, cancel := utils.CombinedContext(10*time.Second, p.t.chStop)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "AUTHORIZE", p.DialInfo().DialAddr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Challenge", hex.EncodeToString(challengeMsg))

	resp, err := p.t.doRequest(req)
	if err != nil {
		return errors.Wrapf(err, "error verifying peer address (%v)", p.DialInfo().DialAddr)
	}
	p.stream.ReadCloser = resp.Body
	return nil
}

func (p *peerConn) ReceiveChallengeIdentityResponse() (_ []swarm.ChallengeIdentityResponse, err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	var verifyResp []swarm.ChallengeIdentityResponse
	err = json.NewDecoder(p.stream.ReadCloser).Decode(&verifyResp)
	if err != nil {
		return nil, err
	}
	return verifyResp, nil
}

func (p *peerConn) RespondChallengeIdentity(verifyAddressResponse []swarm.ChallengeIdentityResponse) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	err = json.NewEncoder(p.stream.Writer).Encode(verifyAddressResponse)
	if err != nil {
		http.Error(p.stream.Writer.(http.ResponseWriter), err.Error(), http.StatusInternalServerError)
		return err
	}
	return nil
}

func (p *peerConn) FetchRef(refID types.RefID) error {
	return types.ErrUnimplemented
}

func (p *peerConn) SendRefHeader(haveBlob bool) error {
	return types.ErrUnimplemented
}

func (p *peerConn) SendRefPacket(data []byte, end bool) error {
	return types.ErrUnimplemented
}

func (p *peerConn) ReceiveRefHeader() (swarm.FetchRefResponseHeader, error) {
	return swarm.FetchRefResponseHeader{}, types.ErrUnimplemented
}

func (p *peerConn) ReceiveRefPacket() (swarm.FetchRefResponseBody, error) {
	return swarm.FetchRefResponseBody{}, types.ErrUnimplemented
}

func (p *peerConn) AnnouncePeers(ctx context.Context, peerDialInfos []swarm.PeerDialInfo) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		p.t.Warn("peer has no DialAddr")
		return nil
	}

	ctx, cancel := utils.CombinedContext(ctx, 10*time.Second, p.t.chStop)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", p.DialInfo().DialAddr, nil)
	if err != nil {
		return err
	}

	resp, err := p.t.doRequest(req)
	if err != nil {
		return errors.Wrapf(err, "error announcing peers to peer (%v)", p.DialInfo().DialAddr)
	}
	defer resp.Body.Close()
	return nil
}

func (p *peerConn) Close() error {
	if p.stream.ReadCloser != nil {
		return p.stream.ReadCloser.Close()
	}
	return nil
}