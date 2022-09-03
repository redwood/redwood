package rpc

import (
	"bytes"
	"context"
	"crypto/rand"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	ma "github.com/multiformats/go-multiaddr"

	"redwood.dev/blob"
	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/swarm"
	"redwood.dev/swarm/libp2p"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/protohush"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
	"redwood.dev/utils/authutils"
	. "redwood.dev/utils/generics"
)

type HTTPConfig struct {
	Enabled     bool                                      `json:"enabled"     yaml:"Enabled"`
	ListenHost  string                                    `json:"listenHost"  yaml:"ListenHost"`
	TLSCertFile string                                    `json:"tlsCertFile" yaml:"TLSCertFile"`
	TLSKeyFile  string                                    `json:"tlsKeyFile"  yaml:"TLSKeyFile"`
	Whitelist   HTTPWhitelistConfig                       `json:"whitelist"   yaml:"Whitelist"`
	Server      func(innerServer *HTTPServer) interface{} `json:"-"           yaml:"-"`
}

type HTTPWhitelistConfig struct {
	Enabled        bool            `json:"enabled"        yaml:"Enabled"`
	PermittedAddrs []types.Address `json:"permittedAddrs" yaml:"PermittedAddrs"`
}

func StartHTTPRPC(svc interface{}, config *HTTPConfig, jwtSecret []byte, jwtExpiry time.Duration) (*http.Server, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}

	httpServer := &http.Server{
		Addr: config.ListenHost,
	}

	go func() {
		server := rpc.NewServer()
		server.RegisterCodec(json2.NewCodec(), "application/json")
		server.RegisterService(svc, "RPC")

		httpServer.Handler = server
		if config.Whitelist.Enabled {
			httpServer.Handler = NewWhitelistMiddleware(config.Whitelist.PermittedAddrs, jwtSecret, jwtExpiry, server)
		}
		httpServer.Handler = utils.UnrestrictedCors(httpServer.Handler)
		httpServer.ListenAndServe()
	}()

	return httpServer, nil
}

type HTTPServer struct {
	log.Logger
	jwtSecret       []byte
	authProto       protoauth.AuthProtocol
	blobProto       protoblob.BlobProtocol
	hushProto       protohush.HushProtocol
	treeProto       prototree.TreeProtocol
	peerStore       swarm.PeerStore
	keyStore        identity.KeyStore
	blobStore       blob.Store
	controllerHub   tree.ControllerHub
	libp2pTransport libp2p.Transport
}

func NewHTTPServer(
	jwtSecret []byte,
	authProto protoauth.AuthProtocol,
	blobProto protoblob.BlobProtocol,
	hushProto protohush.HushProtocol,
	treeProto prototree.TreeProtocol,
	peerStore swarm.PeerStore,
	keyStore identity.KeyStore,
	blobStore blob.Store,
	controllerHub tree.ControllerHub,
	libp2pTransport libp2p.Transport,
) *HTTPServer {
	return &HTTPServer{
		Logger:          log.NewLogger("http rpc"),
		jwtSecret:       jwtSecret,
		authProto:       authProto,
		blobProto:       blobProto,
		hushProto:       hushProto,
		treeProto:       treeProto,
		peerStore:       peerStore,
		keyStore:        keyStore,
		blobStore:       blobStore,
		controllerHub:   controllerHub,
		libp2pTransport: libp2pTransport,
	}
}

type (
	UcanArgs     struct{}
	UcanResponse struct {
		JWT string
	}
)

func (s *HTTPServer) Ucan(r *http.Request, args *UcanArgs, resp *UcanResponse) error {
	identity, err := s.keyStore.DefaultPublicIdentity()
	if err != nil {
		return err
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"addresses": []string{identity.Address().Hex()},
		"nbf":       time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	jwtTokenString, err := jwtToken.SignedString(s.jwtSecret)
	if err != nil {
		return err
	}

	resp.JWT = jwtTokenString
	return nil
}

type (
	SubscribeArgs struct {
		StateURI string
		Txs      bool
		States   bool
		Keypath  string
	}
	SubscribeResponse struct{}
)

func (s *HTTPServer) Subscribe(r *http.Request, args *SubscribeArgs, resp *SubscribeResponse) (err error) {
	if s.treeProto == nil {
		return errors.ErrUnsupported
	} else if args.StateURI == "" {
		return errors.New("missing StateURI")
	}

	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)

	var subscriptionType prototree.SubscriptionType
	if args.Txs {
		subscriptionType |= prototree.SubscriptionType_Txs
	}
	if args.States {
		subscriptionType |= prototree.SubscriptionType_States
	}

	err = s.treeProto.Subscribe(ctx, args.StateURI)
	if err != nil {
		return errors.Wrap(err, "error subscribing to "+args.StateURI)
	}
	return nil
}

type (
	IdentitiesArgs     struct{}
	IdentitiesResponse struct {
		Identities []Identity
	}
	Identity struct {
		Address types.Address
		Public  bool
	}
)

func (s *HTTPServer) Identities(r *http.Request, args *IdentitiesArgs, resp *IdentitiesResponse) error {
	if s.keyStore == nil {
		return errors.ErrUnsupported
	}

	identities, err := s.keyStore.Identities()
	if err != nil {
		return err
	}
	for _, identity := range identities {
		resp.Identities = append(resp.Identities, Identity{identity.Address(), identity.Public})
	}
	return nil
}

type (
	NewIdentityArgs struct {
		Public bool
	}
	NewIdentityResponse struct {
		Address types.Address
	}
)

func (s *HTTPServer) NewIdentity(r *http.Request, args *NewIdentityArgs, resp *NewIdentityResponse) error {
	if s.keyStore == nil {
		return errors.ErrUnsupported
	}
	identity, err := s.keyStore.NewIdentity(args.Public)
	if err != nil {
		return err
	}
	resp.Address = identity.Address()
	return nil
}

type (
	AddPeerArgs struct {
		TransportName string
		DialAddr      string
	}
	AddPeerResponse struct{}
)

func (s *HTTPServer) AddPeer(r *http.Request, args *AddPeerArgs, resp *AddPeerResponse) error {
	if s.peerStore == nil {
		return errors.ErrUnsupported
	}
	s.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName: args.TransportName, DialAddr: args.DialAddr}, "")
	return nil
}

type (
	StaticRelaysArgs     struct{}
	StaticRelaysResponse struct {
		StaticRelays []string
	}
)

func (s *HTTPServer) StaticRelays(r *http.Request, args *StaticRelaysArgs, resp *StaticRelaysResponse) error {
	if s.libp2pTransport == nil {
		return errors.ErrUnsupported
	}
	resp.StaticRelays = Reduce(s.libp2pTransport.Relays(), func(into []string, rr libp2p.RelayAndReservation) []string {
		return append(into, Map(rr.AddrInfo.Addrs, func(ma ma.Multiaddr) string { return ma.String() })...)
	}, []string{})
	return nil
}

type (
	AddStaticRelayArgs struct {
		DialAddr string
	}
	AddStaticRelayResponse struct{}
)

func (s *HTTPServer) AddStaticRelay(r *http.Request, args *AddStaticRelayArgs, resp *AddStaticRelayResponse) error {
	if s.libp2pTransport == nil {
		return errors.ErrUnsupported
	}
	return s.libp2pTransport.AddRelay(args.DialAddr)
}

type (
	RemoveStaticRelayArgs struct {
		DialAddr string
	}
	RemoveStaticRelayResponse struct{}
)

func (s *HTTPServer) RemoveStaticRelay(r *http.Request, args *RemoveStaticRelayArgs, resp *RemoveStaticRelayResponse) error {
	if s.libp2pTransport == nil {
		return errors.ErrUnsupported
	}
	return s.libp2pTransport.RemoveRelay(args.DialAddr)
}

type (
	VaultsArgs     struct{}
	VaultsResponse struct {
		Vaults []string
	}
)

func (s *HTTPServer) Vaults(r *http.Request, args *VaultsArgs, resp *VaultsResponse) error {
	if s.hushProto == nil {
		return errors.ErrUnsupported
	}

	resp.Vaults = s.hushProto.Vaults().Union(s.treeProto.Vaults()).Slice()
	return nil
}

type (
	AddVaultArgs struct {
		DialAddr string
	}
	AddVaultResponse struct{}
)

func (s *HTTPServer) AddVault(r *http.Request, args *AddVaultArgs, resp *AddVaultResponse) error {
	err := s.treeProto.AddVault(args.DialAddr)
	if err != nil {
		return err
	}
	return s.hushProto.AddVault(args.DialAddr)
}

type (
	RemoveVaultArgs struct {
		DialAddr string
	}
	RemoveVaultResponse struct{}
)

func (s *HTTPServer) RemoveVault(r *http.Request, args *RemoveVaultArgs, resp *RemoveVaultResponse) error {
	err := s.treeProto.RemoveVault(args.DialAddr)
	if err != nil {
		return err
	}
	return s.hushProto.RemoveVault(args.DialAddr)
}

type (
	StateURIsWithDataArgs     struct{}
	StateURIsWithDataResponse struct {
		StateURIs []string
	}
)

func (s *HTTPServer) StateURIsWithData(r *http.Request, args *StateURIsWithDataArgs, resp *StateURIsWithDataResponse) error {
	if s.controllerHub == nil {
		return errors.ErrUnsupported
	}
	stateURIs, err := s.controllerHub.StateURIsWithData()
	if err != nil {
		return err
	}
	resp.StateURIs = stateURIs.Slice()
	return nil
}

type (
	SendTxArgs struct {
		Tx tree.Tx
	}
	SendTxResponse struct{}
)

func (s *HTTPServer) SendTx(r *http.Request, args *SendTxArgs, resp *SendTxResponse) error {
	if s.treeProto == nil {
		return errors.ErrUnsupported
	}
	return s.treeProto.SendTx(context.Background(), args.Tx)
}

type (
	StoreBlobArgs struct {
		Blob []byte
	}
	StoreBlobResponse struct {
		SHA1 types.Hash
		SHA3 types.Hash
	}
)

func (s *HTTPServer) StoreBlob(r *http.Request, args *StoreBlobArgs, resp *StoreBlobResponse) error {
	sha1, sha3, err := s.blobStore.StoreBlob(ioutil.NopCloser(bytes.NewReader(args.Blob)))
	if err != nil {
		return err
	}
	resp.SHA1 = sha1
	resp.SHA3 = sha3
	return nil
}

type (
	PeersArgs struct {
		StateURI string
	}
	PeersResponse struct {
		Peers []Peer
	}
	Peer struct {
		Identities  []PeerIdentity
		Transport   string
		DialAddr    string
		StateURIs   []string
		LastContact uint64
	}
	PeerIdentity struct {
		Address          types.Address
		SigningPublicKey *crypto.SigningPublicKey
		AsymEncPubkey    *crypto.AsymEncPubkey
	}
)

func (s *HTTPServer) Peers(r *http.Request, args *PeersArgs, resp *PeersResponse) error {
	if s.peerStore == nil {
		return errors.ErrUnsupported
	}

	peerstoreAddrs := NewSet[types.Address](nil)

	for _, peer := range s.peerStore.Peers() {
		for _, endpoint := range peer.Endpoints() {
			var identities []PeerIdentity
			for addr := range peer.Addresses() {
				sigpubkey, encpubkey := peer.PublicKeys(addr)
				identities = append(identities, PeerIdentity{
					Address:          addr,
					SigningPublicKey: sigpubkey,
					AsymEncPubkey:    encpubkey,
				})
			}
			var lastContact uint64
			if !endpoint.LastContact().IsZero() {
				lastContact = uint64(endpoint.LastContact().UTC().Unix())
			}
			resp.Peers = append(resp.Peers, Peer{
				Identities:  identities,
				Transport:   endpoint.DialInfo().TransportName,
				DialAddr:    endpoint.DialInfo().DialAddr,
				StateURIs:   peer.StateURIs().Slice(),
				LastContact: lastContact,
			})
			peerstoreAddrs = peerstoreAddrs.Union(peer.Addresses())
		}
	}

	pubkeyAddrs, err := s.hushProto.PubkeyBundleAddresses()
	if err != nil {
		return err
	}
	for _, addr := range pubkeyAddrs {
		if !peerstoreAddrs.Contains(addr) {
			resp.Peers = append(resp.Peers, Peer{
				Identities: []PeerIdentity{{Address: addr}},
			})
		}
	}
	return nil
}

type whitelistMiddleware struct {
	nextHandler   http.Handler
	accessControl *authutils.UcanAccessControl[capability]
}

type capability uint8

const (
	forbidden capability = iota
	allowed
)

func NewWhitelistMiddleware(permittedAddrs []types.Address, jwtSecret []byte, jwtExpiry time.Duration, nextHandler http.Handler) *whitelistMiddleware {
	if len(jwtSecret) == 0 {
		jwtSecret = make([]byte, 64)
		_, err := rand.Read(jwtSecret)
		if err != nil {
			panic(err)
		}
	}

	accessControl := authutils.NewUcanAccessControl[capability](jwtSecret, jwtExpiry, nil)
	for _, addr := range permittedAddrs {
		accessControl.SetCapabilities(addr, []capability{allowed})
	}
	return &whitelistMiddleware{
		nextHandler:   nextHandler,
		accessControl: accessControl,
	}
}

func (mw *whitelistMiddleware) SetAllowed(addr types.Address, isAllowed bool) {
	if isAllowed {
		mw.accessControl.SetCapabilities(addr, []capability{allowed})
	} else {
		mw.accessControl.SetCapabilities(addr, nil)
	}
}

func (mw *whitelistMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Challenge crypto.ChallengeMsg `header:"Challenge"`
		Signature crypto.Signature    `header:"Signature"`
	}
	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Method == "AUTHORIZE" {
		if req.Signature.Length() == 0 {
			// Wants challenge
			challenge, err := mw.accessControl.GenerateChallenge()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			utils.RespondJSON(w, struct {
				Challenge string `json:"challenge"`
			}{challenge.Hex()})

		} else {
			// Has challenge response
			_, ucan, err := mw.accessControl.RespondChallenge([]authutils.ChallengeSignature{{Challenge: req.Challenge, Signature: req.Signature}})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			utils.RespondJSON(w, struct {
				Ucan string `json:"ucan"`
			}{ucan})
		}

	} else {
		isAllowed, err := mw.accessControl.UserHasCapabilityByJWT(r.Header.Get("Authorization"), allowed)
		if err != nil {
			http.Error(w, "bad Authorization header", http.StatusBadRequest)
			return
		}
		if !isAllowed {
			http.Error(w, "nope", http.StatusForbidden)
			return
		}
		mw.nextHandler.ServeHTTP(w, r)
	}
}
