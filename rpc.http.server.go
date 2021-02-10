package redwood

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

	"redwood.dev/ctx"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type HTTPRPCService interface {
	ctx.Logger
	CtxStart(onStartup func() error, onAboutToStop func(), onChildAboutToStop func(inChild ctx.Ctx), onStopping func()) error
}

type WhitelistConfig struct {
	Enabled        bool
	PermittedAddrs []types.Address
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
			challenge, err := types.GenerateChallengeMsg()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			mw.pendingAuthorizationsMu.Lock()
			defer mw.pendingAuthorizationsMu.Unlock()

			mw.pendingAuthorizations[string(challenge)] = struct{}{}

			challengeHex := hex.EncodeToString(challenge)

			respondJSON(w, struct {
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

			sigpubkey, err := RecoverSigningPubkey(types.HashBytes(challenge), sig)
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

			respondJSON(w, struct {
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

func StartHTTPRPC(svc HTTPRPCService, config *HTTPRPCConfig) error {
	if config == nil || !config.Enabled {
		return nil
	}
	return svc.CtxStart(
		// on startup
		func() error {
			svc.SetLogLabel("rpc")
			svc.Infof(0, "rpc server listening on %v", config.ListenHost)

			go func() {
				server := rpc.NewServer()
				server.RegisterCodec(json2.NewCodec(), "application/json")
				server.RegisterService(svc, "RPC")

				var handler http.Handler = server
				if config.Whitelist.Enabled {
					handler = NewWhitelistMiddleware(config.Whitelist.PermittedAddrs, server)
				}
				handler = UnrestrictedCors(handler)

				http.ListenAndServe(config.ListenHost, handler)
			}()

			return nil
		},
		nil,
		nil,
		// on shutdown
		nil,
	)
}

type HTTPRPCServer struct {
	*ctx.Context
	address types.Address
	host    Host
}

func NewHTTPRPCServer(addr types.Address, host Host) *HTTPRPCServer {
	return &HTTPRPCServer{
		Context: &ctx.Context{},
		address: addr,
		host:    host,
	}
}

type (
	RPCSubscribeArgs struct {
		StateURI string
		Txs      bool
		States   bool
		Keypath  string
	}
	RPCSubscribeResponse struct{}
)

func (s *HTTPRPCServer) Subscribe(r *http.Request, args *RPCSubscribeArgs, resp *RPCSubscribeResponse) error {
	if args.StateURI == "" {
		return errors.New("missing StateURI")
	}

	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)

	var subscriptionType SubscriptionType
	if args.Txs {
		subscriptionType |= SubscriptionType_Txs
	}
	if args.States {
		subscriptionType |= SubscriptionType_States
	}

	sub, err := s.host.Subscribe(ctx, args.StateURI, subscriptionType, tree.Keypath(args.Keypath))
	if err != nil {
		return errors.Wrap(err, "error subscribing to "+args.StateURI)
	}
	sub.Close()
	return nil
}

type (
	RPCNodeAddressArgs     struct{}
	RPCNodeAddressResponse struct {
		Address types.Address
	}
)

func (s *HTTPRPCServer) NodeAddress(r *http.Request, args *RPCNodeAddressArgs, resp *RPCNodeAddressResponse) error {
	resp.Address = s.address
	return nil
}

type (
	RPCAddPeerArgs struct {
		TransportName string
		DialAddr      string
	}
	RPCAddPeerResponse struct{}
)

func (s *HTTPRPCServer) AddPeer(r *http.Request, args *RPCAddPeerArgs, resp *RPCAddPeerResponse) error {
	s.host.AddPeer(PeerDialInfo{TransportName: args.TransportName, DialAddr: args.DialAddr})
	return nil
}

type (
	RPCKnownStateURIsArgs     struct{}
	RPCKnownStateURIsResponse struct {
		StateURIs []string
	}
)

func (s *HTTPRPCServer) KnownStateURIs(r *http.Request, args *RPCKnownStateURIsArgs, resp *RPCKnownStateURIsResponse) error {
	stateURIs, err := s.host.Controllers().KnownStateURIs()
	if err != nil {
		return err
	}
	resp.StateURIs = stateURIs
	return nil
}

type (
	RPCSendTxArgs struct {
		Tx Tx
	}
	RPCSendTxResponse struct{}
)

func (s *HTTPRPCServer) SendTx(r *http.Request, args *RPCSendTxArgs, resp *RPCSendTxResponse) error {
	return s.host.SendTx(context.Background(), args.Tx)
}
