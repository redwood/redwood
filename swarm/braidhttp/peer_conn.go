package braidhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"redwood.dev/blob"
	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
)

type peerConn struct {
	swarm.PeerEndpoint

	t                    *transport
	sessionID            types.ID
	addressesFromRequest Set[types.Address]
	myUcanWithPeer       string

	// stream
	stream struct {
		io.ReadCloser
		http.ResponseWriter
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

func (p *peerConn) SendEncryptedTx(ctx context.Context, encryptedTx prototree.EncryptedTx) (err error) {
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

func (p *peerConn) Ack(ctx context.Context, stateURI string, txID state.Version) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		return nil
	}

	txIDBytes, err := txID.MarshalText()
	if err != nil {
		return errors.WithStack(err)
	}

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

func (p *peerConn) ChallengeIdentity(ctx context.Context, challengeMsg crypto.ChallengeMsg) (err error) {
	defer errors.AddStack(&err)
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.stream.ReadCloser == nil {
		// Outgoing request
		req, err := http.NewRequestWithContext(ctx, "AUTHORIZE", p.DialInfo().DialAddr, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Challenge", challengeMsg.Hex())

		resp, err := p.doRequest(req)
		if err != nil {
			return errors.Wrapf(err, "error verifying peer address (%v)", p.DialInfo().DialAddr)
		}
		defer resp.Body.Close()

		p.stream.ReadCloser = resp.Body

		var response []protoauth.ChallengeIdentityResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			return err
		}
		return p.t.HandleIncomingIdentityChallengeResponse(response, p)

	} else {
		// Incoming request
		p.stream.ResponseWriter.Header().Set("Challenge", challengeMsg.Hex())
	}
	return nil
}

func (p *peerConn) RespondChallengeIdentity(ctx context.Context, verifyAddressResponse []protoauth.ChallengeIdentityResponse) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	p.stream.ResponseWriter.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(p.stream.ResponseWriter).Encode(verifyAddressResponse)
	if err != nil {
		http.Error(p.stream.ResponseWriter, err.Error(), http.StatusInternalServerError)
		return err
	}
	return nil
}

func (p *peerConn) SendUcan(ctx context.Context, ucan string) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.stream.ReadCloser == nil {
		// Outgoing request
		req, err := http.NewRequestWithContext(ctx, "AUTHORIZE", p.DialInfo().DialAddr, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Ucan", ucan)

		resp, err := p.doRequest(req)
		if err != nil {
			return errors.Wrapf(err, "error verifying peer address (%v)", p.DialInfo().DialAddr)
		}
		defer resp.Body.Close()

	} else {
		// Incoming request
		p.stream.ResponseWriter.Header().Set("Ucan", ucan)
	}

	return nil
}

func (p *peerConn) FetchBlobManifest(ctx context.Context, blobID blob.ID) (blob.Manifest, error) {
	return blob.Manifest{}, errors.ErrUnimplemented
}

func (p *peerConn) SendBlobManifest(ctx context.Context, m blob.Manifest, exists bool) error {
	return errors.ErrUnimplemented
}

func (p *peerConn) FetchBlobChunk(ctx context.Context, sha3 types.Hash) ([]byte, error) {
	return nil, errors.ErrUnimplemented
}

func (p *peerConn) SendBlobChunk(ctx context.Context, chunk []byte, exists bool) error {
	return errors.ErrUnimplemented
}

func (p *peerConn) AnnouncePeers(ctx context.Context, peerDialInfos []swarm.PeerDialInfo) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

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
	if p.myUcanWithPeer != "" {
		req.Header.Set("Authorization", "Bearer "+p.myUcanWithPeer)
	}

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
