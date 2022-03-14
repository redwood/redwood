package rpc

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"

	"redwood.dev/blob"
	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/libp2p"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type HTTPConfig struct {
	Enabled     bool                                      `json:"enabled"    yaml:"Enabled"`
	ListenHost  string                                    `json:"listenHost" yaml:"ListenHost"`
	TLSCertFile string                                    `json:"tlsCertFile" yaml:"TLSCertFile"`
	TLSKeyFile  string                                    `json:"tlsKeyFile" yaml:"TLSKeyFile"`
	Whitelist   HTTPWhitelistConfig                       `json:"whitelist"  yaml:"Whitelist"`
	Server      func(innerServer *HTTPServer) interface{} `json:"-"          yaml:"-"`
}

type HTTPWhitelistConfig struct {
	Enabled        bool            `json:"enabled"        yaml:"Enabled"`
	PermittedAddrs []types.Address `json:"permittedAddrs" yaml:"PermittedAddrs"`
}

func StartHTTPRPC(svc interface{}, config *HTTPConfig, jwtSecret []byte) (*http.Server, error) {
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
			httpServer.Handler = NewWhitelistMiddleware(config.Whitelist.PermittedAddrs, jwtSecret, server)
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
		"address": identity.Address().Hex(),
		"nbf":     time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
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
	for _, addr := range s.libp2pTransport.StaticRelays().MultiaddrStrings() {
		resp.StaticRelays = append(resp.StaticRelays, addr)
	}
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
	return s.libp2pTransport.AddStaticRelay(args.DialAddr)
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
	return s.libp2pTransport.RemoveStaticRelay(args.DialAddr)
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
	PrivateTreeMembersArgs struct {
		StateURI string
	}
	PrivateTreeMembersResponse struct {
		Members []types.Address
	}
)

func (s *HTTPServer) PrivateTreeMembers(r *http.Request, args *PrivateTreeMembersArgs, resp *PrivateTreeMembersResponse) error {
	if s.controllerHub == nil {
		return errors.ErrUnsupported
	}
	node, err := s.controllerHub.StateAtVersion(args.StateURI, nil)
	if err != nil {
		return err
	}
	defer node.Close()

	subkeys := node.NodeAt(state.Keypath("Members"), nil).Subkeys()
	var addrs []types.Address
	for _, k := range subkeys {
		addr, err := types.AddressFromHex(string(k))
		if err != nil {
			continue
		}
		addrs = append(addrs, addr)
	}
	resp.Members = addrs

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
		}
	}
	return nil
}

type whitelistMiddleware struct {
	permittedAddrs          map[types.Address]struct{}
	nextHandler             http.Handler
	jwtSecret               []byte
	pendingAuthorizations   map[string]struct{}
	pendingAuthorizationsMu sync.Mutex
}

func NewWhitelistMiddleware(permittedAddrs []types.Address, jwtSecret []byte, nextHandler http.Handler) *whitelistMiddleware {
	if len(jwtSecret) == 0 {
		jwtSecret = make([]byte, 64)
		_, err := rand.Read(jwtSecret)
		if err != nil {
			panic(err)
		}
	}
	paddrs := make(map[types.Address]struct{}, len(permittedAddrs))
	for _, addr := range permittedAddrs {
		paddrs[addr] = struct{}{}
	}
	return &whitelistMiddleware{
		permittedAddrs:        paddrs,
		nextHandler:           nextHandler,
		jwtSecret:             jwtSecret,
		pendingAuthorizations: make(map[string]struct{}),
	}
}

func (mw *whitelistMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "AUTHORIZE" {
		responseHex := r.Header.Get("Response")
		if responseHex == "" {
			// Wants challenge
			challenge, err := protoauth.GenerateChallengeMsg()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			mw.pendingAuthorizationsMu.Lock()
			defer mw.pendingAuthorizationsMu.Unlock()

			mw.pendingAuthorizations[string(challenge)] = struct{}{}

			challengeHex := hex.EncodeToString(challenge)

			utils.RespondJSON(w, struct {
				Challenge string `json:"challenge"`
			}{challengeHex})

		} else {
			// Has challenge response
			challengeHex := r.Header.Get("Challenge")
			if challengeHex == "" {
				http.Error(w, "must provide Challenge header", http.StatusBadRequest)
				return
			}

			challenge, err := hex.DecodeString(challengeHex)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			mw.pendingAuthorizationsMu.Lock()
			defer mw.pendingAuthorizationsMu.Unlock()
			_, exists := mw.pendingAuthorizations[string(challenge)]
			if !exists {
				http.Error(w, "no pending authorization", http.StatusBadRequest)
				return
			}

			sig, err := hex.DecodeString(responseHex)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			sigpubkey, err := crypto.RecoverSigningPubkey(types.HashBytes(challenge), sig)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			delete(mw.pendingAuthorizations, string(challenge)) // @@TODO: expiration/garbage collection for failed auths

			jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"address": sigpubkey.Address().Hex(),
				"nbf":     time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
			})

			// Sign and get the complete encoded token as a string using the secret
			jwtTokenString, err := jwtToken.SignedString(mw.jwtSecret)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			utils.RespondJSON(w, struct {
				JWT string `json:"jwt"`
			}{jwtTokenString})
		}

	} else {
		claims, exists, err := utils.ParseJWT(r.Header.Get("Authorization"), mw.jwtSecret)
		if err != nil {
			http.Error(w, "bad Authorization header", http.StatusBadRequest)
			return
		} else if !exists {
			http.Error(w, "no JWT present", http.StatusForbidden)
			return
		}
		addrHex, ok := claims["address"].(string)
		if !ok {
			http.Error(w, "jwt does not contain 'address' claim", http.StatusBadRequest)
			return
		}
		addr, err := types.AddressFromHex(addrHex)
		if err != nil {
			http.Error(w, "jwt 'address' claim contains invalid data", http.StatusBadRequest)
			return
		}
		_, exists = mw.permittedAddrs[addr]
		if !exists {
			http.Error(w, "nope", http.StatusForbidden)
			return
		}

		mw.nextHandler.ServeHTTP(w, r)
	}
}
