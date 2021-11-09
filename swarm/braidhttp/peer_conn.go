package braidhttp

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type peerConn struct {
	swarm.PeerEndpoint

	t         *transport
	sessionID types.ID

	// stream
	stream struct {
		io.ReadCloser
		io.Writer
		http.Flusher
	}
}

var (
	_ protoauth.AuthPeerConn = (*peerConn)(nil)
	_ protoblob.BlobPeerConn = (*peerConn)(nil)
	_ prototree.TreePeerConn = (*peerConn)(nil)
)

func (p *peerConn) Transport() swarm.Transport {
	return p.t
}

func (p *peerConn) EnsureConnected(ctx context.Context) error {
	return nil
}

func (p *peerConn) Subscribe(ctx context.Context, stateURI string) (_ prototree.ReadableSubscription, err error) {
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

	subTypeBytes, err := prototree.SubscriptionType_Txs.MarshalText()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Subscribe", string(subTypeBytes))

	resp, err := p.t.doRequest(req)
	if err != nil {
		return nil, errors.Wrapf(err, "error subscribing to peer (%v) (state URI: %v)", p.DialInfo().DialAddr, stateURI)
	}
	defer resp.Body.Close()

	p.t.storeAltSvcHeaderPeers(resp.Header)

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.Errorf("error subscribing to peer (%v) (state URI: %v): %v", p.DialInfo().DialAddr, stateURI, string(bodyBytes))
	}
	return &httpReadableSubscription{
		peer:    p,
		stream:  resp.Body,
		private: resp.Header.Get("Private") == "true",
	}, nil
}

func (p *peerConn) SendTx(ctx context.Context, tx tree.Tx) (err error) {
	if !p.Dialable() || !p.Ready() {
		return nil
	}

	defer func() { p.UpdateConnStats(err == nil) }()

	ctx, cancel := utils.CombinedContext(ctx, p.t.Ctx(), 10*time.Second)
	defer cancel()

	req, err := putRequestFromTx(ctx, tx, p.DialInfo().DialAddr)
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

func (p *peerConn) SendPrivateTx(ctx context.Context, encryptedTx prototree.EncryptedTx) (err error) {
	if !p.Dialable() || !p.Ready() {
		return nil
	}

	defer func() { p.UpdateConnStats(err == nil) }()

	ctx, cancel := utils.CombinedContext(ctx, p.t.Ctx(), 10*time.Second)
	defer cancel()

	var body bytes.Buffer
	err = json.NewEncoder(&body).Encode(encryptedTx)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", p.DialInfo().DialAddr, &body)
	if err != nil {
		return errors.WithStack(err)
	}
	req.Header.Set("Private", "true")

	resp, err := p.t.doRequest(req)
	if err != nil {
		return errors.Wrapf(err, "error PUTting tx to peer (%v)", p.DialInfo().DialAddr)
	}
	defer resp.Body.Close()
	return nil
}

func (p *peerConn) Ack(stateURI string, txID state.Version) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		p.t.Warn("peer has no DialAddr")
		return nil
	}

	txIDBytes, err := txID.MarshalText()
	if err != nil {
		return errors.WithStack(err)
	}

	ctx, cancel := context.WithTimeout(p.t.Ctx(), 10*time.Second)
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

func (p *peerConn) AnnounceP2PStateURI(ctx context.Context, stateURI string) (err error) {
	// @@TODO?
	return nil
}

func (p *peerConn) ChallengeIdentity(challengeMsg protoauth.ChallengeMsg) (err error) {
	defer errors.AddStack(&err)
	defer func() { p.UpdateConnStats(err == nil) }()

	ctx, cancel := context.WithTimeout(p.t.Ctx(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "AUTHORIZE", p.DialInfo().DialAddr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Challenge", hex.EncodeToString(challengeMsg))

	resp, err := p.doRequest(req)
	if err != nil {
		return errors.Wrapf(err, "error verifying peer address (%v)", p.DialInfo().DialAddr)
	}

	p.stream.ReadCloser = resp.Body
	return nil
}

func (p *peerConn) ReceiveChallengeIdentityResponse() (_ []protoauth.ChallengeIdentityResponse, err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	var verifyResp []protoauth.ChallengeIdentityResponse
	err = json.NewDecoder(p.stream.ReadCloser).Decode(&verifyResp)
	if err != nil {
		return nil, err
	}
	return verifyResp, nil
}

func (p *peerConn) RespondChallengeIdentity(verifyAddressResponse []protoauth.ChallengeIdentityResponse) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	err = json.NewEncoder(p.stream.Writer).Encode(verifyAddressResponse)
	if err != nil {
		http.Error(p.stream.Writer.(http.ResponseWriter), err.Error(), http.StatusInternalServerError)
		return err
	}
	return nil
}

func (p *peerConn) FetchBlobManifest(blobID blob.ID) (blob.Manifest, error) {
	return blob.Manifest{}, errors.ErrUnimplemented
}

func (p *peerConn) SendBlobManifest(m blob.Manifest, exists bool) error {
	return errors.ErrUnimplemented
}

func (p *peerConn) FetchBlobChunk(sha3 types.Hash) ([]byte, error) {
	return nil, errors.ErrUnimplemented
}

func (p *peerConn) SendBlobChunk(chunk []byte, exists bool) error {
	return errors.ErrUnimplemented
}

func (p *peerConn) AnnouncePeers(ctx context.Context, peerDialInfos []swarm.PeerDialInfo) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		p.t.Warn("peer has no DialAddr")
		return nil
	}

	ctx, cancel := context.WithTimeout(p.t.Ctx(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", p.DialInfo().DialAddr, nil)
	if err != nil {
		return err
	}

	resp, err := p.doRequest(req)
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

func (p *peerConn) doRequest(req *http.Request) (*http.Response, error) {
	resp, err := p.t.doRequest(req)
	if err != nil {
		return nil, err
	}
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		duID := utils.DeviceIDFromX509Pubkey(resp.TLS.PeerCertificates[0].PublicKey)
		p.SetDeviceUniqueID(duID)
	}
	return resp, nil
}
