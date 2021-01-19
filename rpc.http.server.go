package redwood

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type HTTPRPCServer interface {
	Ctx() *ctx.Context
	Start() error
}

type httpRPCServer struct {
	*ctx.Context

	address    types.Address
	listenAddr string
	host       Host
}

func NewHTTPRPCServer(addr types.Address, listenAddr string, host Host) HTTPRPCServer {
	return &httpRPCServer{
		Context:    &ctx.Context{},
		address:    addr,
		listenAddr: listenAddr,
		host:       host,
	}
}

func (s *httpRPCServer) Start() error {
	return s.CtxStart(
		// on startup
		func() error {
			s.SetLogLabel("rpc")
			s.Infof(0, "rpc server listening on %v", s.listenAddr)

			go func() {
				server := rpc.NewServer()
				server.RegisterCodec(json2.NewCodec(), "application/json")
				server.RegisterService(s, "RPC")
				http.ListenAndServe(s.listenAddr, UnrestrictedCors(server))
			}()

			return nil
		},
		nil,
		nil,
		// on shutdown
		nil,
	)
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

func (s *httpRPCServer) Subscribe(r *http.Request, args *SubscribeArgs, resp *SubscribeResponse) error {
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

func (s *httpRPCServer) NodeAddress(r *http.Request, args *NodeAddressArgs, resp *NodeAddressResponse) error {
	resp.Address = s.address
	return nil
}

type (
	KnownStateURIsArgs     struct{}
	KnownStateURIsResponse struct {
		StateURIs []string
	}
)

func (s *httpRPCServer) KnownStateURIs(r *http.Request, args *KnownStateURIsArgs, resp *KnownStateURIsResponse) error {
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

func (s *httpRPCServer) SendTx(r *http.Request, args *SendTxArgs, resp *SendTxResponse) error {
	return s.host.SendTx(context.Background(), args.Tx)
}
