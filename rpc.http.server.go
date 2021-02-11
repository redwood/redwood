package redwood

import (
	"context"
	"net/http"
	"time"

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
	ListenAddr() string
}

func StartHTTPRPC(svc HTTPRPCService) error {
	return svc.CtxStart(
		// on startup
		func() error {
			svc.SetLogLabel("rpc")
			svc.Infof(0, "rpc server listening on %v", svc.ListenAddr())

			go func() {
				server := rpc.NewServer()
				server.RegisterCodec(json2.NewCodec(), "application/json")
				server.RegisterService(svc, "RPC")
				http.ListenAndServe(svc.ListenAddr(), UnrestrictedCors(server))
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

	address    types.Address
	listenAddr string
	host       Host
}

func NewHTTPRPCServer(addr types.Address, listenAddr string, host Host) *HTTPRPCServer {
	return &HTTPRPCServer{
		Context:    &ctx.Context{},
		address:    addr,
		listenAddr: listenAddr,
		host:       host,
	}
}

func (s *HTTPRPCServer) ListenAddr() string {
	return s.listenAddr
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

func (s *HTTPRPCServer) Subscribe(r *http.Request, args *SubscribeArgs, resp *SubscribeResponse) error {
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
	NodeAddressArgs     struct{}
	NodeAddressResponse struct {
		Address types.Address
	}
)

func (s *HTTPRPCServer) NodeAddress(r *http.Request, args *NodeAddressArgs, resp *NodeAddressResponse) error {
	resp.Address = s.address
	return nil
}

type (
	AddPeerArgs struct {
		TransportName string
		DialAddr      string
	}
	AddPeerResponse struct{}
)

func (s *HTTPRPCServer) AddPeer(r *http.Request, args *AddPeerArgs, resp *AddPeerResponse) error {
	s.host.AddPeer(PeerDialInfo{TransportName: args.TransportName, DialAddr: args.DialAddr})
	return nil
}

type (
	KnownStateURIsArgs     struct{}
	KnownStateURIsResponse struct {
		StateURIs []string
	}
)

func (s *HTTPRPCServer) KnownStateURIs(r *http.Request, args *KnownStateURIsArgs, resp *KnownStateURIsResponse) error {
	stateURIs, err := s.host.Controllers().KnownStateURIs()
	if err != nil {
		return err
	}
	resp.StateURIs = stateURIs
	return nil
}

type (
	SendTxArgs struct {
		Tx Tx
	}
	SendTxResponse struct{}
)

func (s *HTTPRPCServer) SendTx(r *http.Request, args *SendTxArgs, resp *SendTxResponse) error {
	return s.host.SendTx(context.Background(), args.Tx)
}
