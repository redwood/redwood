package rpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/pkg/errors"

	"redwood.dev/config"
	"redwood.dev/crypto"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

func StartHTTPRPC(svc interface{}, config *config.HTTPRPCConfig) (*http.Server, error) {
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
			httpServer.Handler = NewWhitelistMiddleware(config.Whitelist.PermittedAddrs, server)
		}
		httpServer.Handler = utils.UnrestrictedCors(httpServer.Handler)

		httpServer.ListenAndServe()
	}()

	return httpServer, nil
}

type HTTPServer struct {
	log.Logger
	authProto     protoauth.AuthProtocol
	blobProto     protoblob.BlobProtocol
	treeProto     prototree.TreeProtocol
	peerStore     swarm.PeerStore
	keyStore      identity.KeyStore
	controllerHub tree.ControllerHub
}

func NewHTTPServer(
	authProto protoauth.AuthProtocol,
	blobProto protoblob.BlobProtocol,
	treeProto prototree.TreeProtocol,
	peerStore swarm.PeerStore,
	keyStore identity.KeyStore,
	controllerHub tree.ControllerHub,
) *HTTPServer {
	return &HTTPServer{
		Logger:        log.NewLogger("http rpc"),
		authProto:     authProto,
		blobProto:     blobProto,
		treeProto:     treeProto,
		peerStore:     peerStore,
		keyStore:      keyStore,
		controllerHub: controllerHub,
	}
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
	defer func() {
		log.NewLogger("RPC").Errorf("SUBSCRIBE err %v", err)
	}()
	log.NewLogger("RPC").Successf("SUBSCRIBE")

	if s.treeProto == nil {
		return types.ErrUnsupported
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

	sub, err := s.treeProto.Subscribe(ctx, args.StateURI, subscriptionType, state.Keypath(args.Keypath), nil)
	if err != nil {
		return errors.Wrap(err, "error subscribing to "+args.StateURI)
	}
	sub.Close()
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
		return types.ErrUnsupported
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
		return types.ErrUnsupported
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
		return types.ErrUnsupported
	}
	s.peerStore.AddDialInfos([]swarm.PeerDialInfo{{TransportName: args.TransportName, DialAddr: args.DialAddr}})
	return nil
}

type (
	KnownStateURIsArgs     struct{}
	KnownStateURIsResponse struct {
		StateURIs []string
	}
)

func (s *HTTPServer) KnownStateURIs(r *http.Request, args *KnownStateURIsArgs, resp *KnownStateURIsResponse) error {
	if s.controllerHub == nil {
		return types.ErrUnsupported
	}
	stateURIs, err := s.controllerHub.KnownStateURIs()
	if err != nil {
		return err
	}
	resp.StateURIs = stateURIs
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
		return types.ErrUnsupported
	}
	return s.treeProto.SendTx(context.Background(), args.Tx)
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
		return types.ErrUnsupported
	}
	members, err := s.controllerHub.Members(args.StateURI)
	if err != nil {
		return err
	}
	resp.Members = members
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
		Address             types.Address
		SigningPublicKey    crypto.SigningPublicKey
		EncryptingPublicKey crypto.EncryptingPublicKey
	}
)

func (s *HTTPServer) Peers(r *http.Request, args *PeersArgs, resp *PeersResponse) error {
	if s.peerStore == nil {
		return types.ErrUnsupported
	}
	for _, peer := range s.peerStore.Peers() {
		var identities []PeerIdentity
		for _, addr := range peer.Addresses() {
			sigpubkey, encpubkey := peer.PublicKeys(addr)
			identities = append(identities, PeerIdentity{
				Address:             addr,
				SigningPublicKey:    sigpubkey,
				EncryptingPublicKey: encpubkey,
			})
		}
		var lastContact uint64
		if !peer.LastContact().IsZero() {
			lastContact = uint64(peer.LastContact().UTC().Unix())
		}
		resp.Peers = append(resp.Peers, Peer{
			Identities:  identities,
			Transport:   peer.DialInfo().TransportName,
			DialAddr:    peer.DialInfo().DialAddr,
			StateURIs:   peer.StateURIs().Slice(),
			LastContact: lastContact,
		})
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

func NewWhitelistMiddleware(permittedAddrs []types.Address, nextHandler http.Handler) *whitelistMiddleware {
	jwtSecret := make([]byte, 64)
	_, err := rand.Read(jwtSecret)
	if err != nil {
		panic(err)
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
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "bad Authorization header", http.StatusBadRequest)
			return
		}

		jwtToken := strings.TrimSpace(authHeader[len("Bearer "):])

		token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}
			return mw.jwtSecret, nil
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			http.Error(w, "invalid jwt token", http.StatusBadRequest)
			return
		}
		addrHex, ok := claims["address"].(string)
		if err != nil {
			http.Error(w, "jwt does not contain 'address' claim", http.StatusBadRequest)
			return
		}
		addr, err := types.AddressFromHex(addrHex)
		if err != nil {
			http.Error(w, "jwt 'address' claim contains invalid data", http.StatusBadRequest)
			return
		}
		_, exists := mw.permittedAddrs[addr]
		if !exists {
			http.Error(w, "nope", http.StatusForbidden)
			return
		}

		mw.nextHandler.ServeHTTP(w, r)
	}
}
