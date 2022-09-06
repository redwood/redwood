package vault

import (
	"bytes"
	"context"
	fmt "fmt"
	"io"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/utils/authutils"
	. "redwood.dev/utils/generics"
)

type Server struct {
	process.Process
	log.Logger
	accessControl *authutils.UcanAccessControl[Capability]
	store         Store
	port          uint64
	jwtSecret     []byte
	server        *grpc.Server
}

var _ VaultRPCServer = (*Server)(nil)

type Config struct {
	JWTSecret               []byte
	JWTExpiry               time.Duration
	Port                    uint64
	DefaultUserCapabilities []Capability
}

func NewServer(config Config, store Store) *Server {
	return &Server{
		Process:       *process.New("rpc server"),
		Logger:        log.NewLogger("rpc server"),
		accessControl: authutils.NewUcanAccessControl(config.JWTSecret, config.JWTExpiry, NewSet(config.DefaultUserCapabilities)),
		store:         store,
		port:          config.Port,
		jwtSecret:     config.JWTSecret,
	}
}

func (s *Server) Start() error {
	err := s.Process.Start()
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", s.port))
	if err != nil {
		return err
	}

	s.server = grpc.NewServer()
	RegisterVaultRPCServer(s.server, s)
	go func() {
		err := s.server.Serve(listener)
		if err != nil {
			s.Errorf("error listening: %v", err)
		}
	}()

	return nil
}

func (s *Server) Close() error {
	// This closes the net.Listener as well.
	s.server.GracefulStop()
	return s.Process.Close()
}

func (s *Server) Authorize(stream VaultRPC_AuthorizeServer) error {
	s.Infof("/authorize")

	challenge, err := s.accessControl.GenerateChallenge()
	if err != nil {
		return status.Errorf(codes.Unknown, "internal server error: %v", err)
	}

	err = stream.Send(&AuthorizeMsg{
		Msg: &AuthorizeMsg_Challenge_{
			Challenge: &AuthorizeMsg_Challenge{
				Challenge: challenge,
			},
		},
	})
	if err != nil {
		return status.Errorf(codes.Unknown, "stream error: %v", err)
	}

	msg, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Unknown, "stream error: %v", err)
	}

	signedChallenge := msg.GetSignedChallenge()
	if signedChallenge == nil {
		return status.Errorf(codes.Unknown, "protocol error")
	}

	ucan, jwtTokenString, err := s.accessControl.RespondChallenge([]authutils.ChallengeSignature{{Challenge: challenge, Signature: signedChallenge.Signature}})
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "bad response: %v", err)
	}

	s.Debugf("authorized %v", ucan.Addresses.Slice())

	err = stream.Send(&AuthorizeMsg{
		Msg: &AuthorizeMsg_Response_{
			Response: &AuthorizeMsg_Response{
				Token: jwtTokenString,
			},
		},
	})
	if err != nil {
		return status.Errorf(codes.Unknown, "stream error: %v", err)
	}
	return nil
}

func (s *Server) Items(ctx context.Context, req *ItemsReq) (*ItemsResp, error) {
	s.Infow("/items", "collection", req.CollectionID, "mtime", time.Unix(int64(req.OldestMtime), 0), "start", req.Start, "end", req.End)

	has, err := s.accessControl.UserHasCapabilityByJWT(req.JWT, Capability_Fetch)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "bad request: %v", err)
	} else if !has {
		return nil, status.Errorf(codes.Unauthenticated, "not allowed")
	}

	items, err := s.store.Items(req.CollectionID, time.Unix(int64(req.OldestMtime), 0), req.Start, req.End)
	if err != nil {
		return nil, status.Errorf(codes.Unknown, "internal server error: %v", err)
	}

	itemIDs := Map(items, func(item Item) string { return item.ItemID })
	return &ItemsResp{ItemIDs: itemIDs}, nil
}

func (s *Server) Fetch(req *FetchReq, stream VaultRPC_FetchServer) error {
	s.Infow("/fetch", "collection", req.CollectionID, "item", req.ItemID)

	has, err := s.accessControl.UserHasCapabilityByJWT(req.JWT, Capability_Fetch)
	if err != nil {
		s.Errorw("/fetch", "collection", req.CollectionID, "item", req.ItemID, "err", err)
		return status.Errorf(codes.Unauthenticated, "bad request: %v", err)
	} else if !has {
		s.Errorw("/fetch", "collection", req.CollectionID, "item", req.ItemID, "err", "not allowed")
		return status.Errorf(codes.Unauthenticated, "not allowed")
	}

	reader, mtime, err := s.store.ReadCloser(req.CollectionID, req.ItemID)
	if errors.Cause(err) == errors.Err404 {
		s.Errorw("/fetch", "collection", req.CollectionID, "item", req.ItemID, "err", "not found")
		return status.Error(codes.Unknown, "not found")
	} else if err != nil {
		s.Errorw("/fetch", "collection", req.CollectionID, "item", req.ItemID, "err", err)
		return status.Errorf(codes.Unknown, "error: %v", err)
	}
	defer reader.Close()

	err = stream.Send(&FetchResp{Mtime: uint64(mtime.UTC().Unix())})
	if err != nil {
		s.Errorw("/fetch", "collection", req.CollectionID, "item", req.ItemID, "err", err)
		return status.Errorf(codes.Unknown, "error: %v", err)
	}

	_, err = io.Copy(FetchServerWriter{stream}, reader)
	if err != nil {
		err = status.Errorf(codes.Unknown, "stream error: %v", err)
		s.Errorw("/fetch", "collection", req.CollectionID, "item", req.ItemID, "err", err)
		return err
	}
	err = stream.Send(&FetchResp{End: true})
	if err != nil {
		err = status.Errorf(codes.Unknown, "stream error: %v", err)
		s.Errorw("/fetch", "collection", req.CollectionID, "item", req.ItemID, "err", err)
		return err
	}
	return nil
}

type FetchServerWriter struct {
	VaultRPC_FetchServer
}

func (w FetchServerWriter) Write(bs []byte) (int, error) {
	err := w.Send(&FetchResp{Data: bs})
	return len(bs), err
}

func (s *Server) Store(stream VaultRPC_StoreServer) error {
	msg, err := stream.Recv()
	if err != nil {
		s.Errorw("/store", "err", err)
		return status.Errorf(codes.Unknown, "stream error: %v", err)
	}

	header := msg.GetHeader()
	if header == nil {
		s.Errorw("/store", "collection", header.CollectionID, "item", header.ItemID, "err", err)
		return status.Errorf(codes.Unknown, "protocol error")
	}

	has, err := s.accessControl.UserHasCapabilityByJWT(header.JWT, Capability_Store)
	if err != nil {
		s.Errorw("/store", "collection", header.CollectionID, "item", header.ItemID, "err", err)
		return status.Errorf(codes.Unauthenticated, "bad request: %v", err)
	} else if !has {
		s.Errorw("/store", "collection", header.CollectionID, "item", header.ItemID, "err", "not allowed")
		return status.Errorf(codes.Unauthenticated, "not allowed")
	}

	reader, writer := io.Pipe()
	go func() {
		var err error
		defer func() { writer.CloseWithError(err) }()

		r := bytes.NewReader(nil)
		for {
			msg, err = stream.Recv()
			if err != nil {
				err = status.Errorf(codes.Unknown, "stream error: %v", err)
				s.Errorw("/store", "collection", header.CollectionID, "item", header.ItemID, "err", err)
				return
			}

			payload := msg.GetPayload()
			if payload == nil {
				err = status.Errorf(codes.Unknown, "protocol error")
				s.Errorw("/store", "collection", header.CollectionID, "item", header.ItemID, "err", err)
				return
			}

			r.Reset(payload.Data)
			_, err = io.Copy(writer, r)
			if err != nil {
				err = status.Errorf(codes.Unknown, "internal server error: %v", err)
				s.Errorw("/store", "collection", header.CollectionID, "item", header.ItemID, "err", err)
				return
			}

			if payload.End {
				break
			}
		}
	}()

	alreadyKnown, err := s.store.Upsert(header.CollectionID, header.ItemID, reader)
	if err != nil {
		err = errors.Wrapf(err, "while upserting item")
		s.Errorw("/store", "collection", header.CollectionID, "item", header.ItemID, "err", err)
		return status.Errorf(codes.Unknown, "internal server error: %v", err)
	}

	if !alreadyKnown {
		s.Successw("/store", "collection", header.CollectionID, "item", header.ItemID)
	} else {
		s.Debugw("/store (ignoring, already known)", "collection", header.CollectionID, "item", header.ItemID)
	}

	err = stream.SendAndClose(&StoreResp{})
	if err != nil {
		err = errors.Wrapf(err, "while sending final packet")
		s.Errorw("/store", "collection", header.CollectionID, "item", header.ItemID, "err", err)
		return status.Errorf(codes.Unknown, "internal server error: %v", err)
	}
	return nil
}

func (s *Server) Delete(ctx context.Context, req *DeleteReq) (*DeleteResp, error) {
	has, err := s.accessControl.UserHasCapabilityByJWT(req.JWT, Capability_Delete)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "bad request: %v", err)
	} else if !has {
		return nil, status.Errorf(codes.Unauthenticated, "not allowed")
	}

	err = s.store.Delete(req.CollectionID, req.ItemID)
	if err != nil {
		return nil, err
	}
	s.Debugw("/delete", "collection", req.CollectionID, "item", req.ItemID)
	return &DeleteResp{}, nil
}

func (s *Server) SetUserCapabilities(ctx context.Context, req *SetUserCapabilitiesReq) (*SetUserCapabilitiesResp, error) {
	s.Infow("/set user capabilities", "address", req.Address.Hex(), "capabilities", req.Capabilities)

	has, err := s.accessControl.UserHasCapabilityByJWT(req.JWT, Capability_Admin)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "bad request: %v", err)
	} else if !has {
		return nil, status.Errorf(codes.Unauthenticated, "not allowed")
	}

	s.accessControl.SetCapabilities(req.Address, req.Capabilities)
	return &SetUserCapabilitiesResp{}, nil
}
