package redwood

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
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

func NewHTTPRPCServer(
	addr types.Address,
	listenAddr string,
	host Host,
) HTTPRPCServer {
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
			s.SetLogLabel(s.address.Pretty() + " rpc")
			s.Infof(0, "rpc server listening on %v", s.listenAddr)

			go func() {
				server := rpc.NewServer()
				server.RegisterCodec(json2.NewCodec(), "application/json")
				server.RegisterService(new(httpRPCServer), "RPC")
				http.ListenAndServe(s.listenAddr, server)
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
	}
	SubscribeResponse struct{}
)

func (s *httpRPCServer) Subscribe(r *http.Request, args *SubscribeArgs, resp *SubscribeResponse) error {
	if args.StateURI == "" {
		return errors.New("missing StateURI")
	}

	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	anySucceeded, err := s.host.Subscribe(ctx, args.StateURI)
	if !anySucceeded {
		return errors.Wrap(err, "all transports failed to subscribe")
	}
	return nil
}

type (
	KnownStateURIsArgs     struct{}
	KnownStateURIsResponse struct {
		StateURIs []string
	}
)

func (s *httpRPCServer) KnownStateURIs(r *http.Request, args *KnownStateURIsArgs, resp *KnownStateURIsResponse) error {
	resp.StateURIs = s.host.Controller().KnownStateURIs()
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
