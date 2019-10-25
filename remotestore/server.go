package remotestore

import (
	"context"
	"net"

	"github.com/dgraph-io/badger"
	"github.com/dgrijalva/jwt-go"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	rw "github.com/brynbellomy/redwood"
	"github.com/brynbellomy/redwood/ctx"
)

type server struct {
	*ctx.Context
	listenNetwork    string
	listenHost       string
	grpc             *grpc.Server
	dbPath           string
	db               *badger.DB
	allowedAddresses map[rw.Address]bool
}

func NewServer(listenNetwork, listenHost, dbPath string, allowedAddresses []rw.Address) *server {
	allowedAddressesMap := map[rw.Address]bool{}
	for _, addr := range allowedAddresses {
		allowedAddressesMap[addr] = true
	}

	return &server{
		Context:          &ctx.Context{},
		listenNetwork:    listenNetwork,
		listenHost:       listenHost,
		grpc:             nil,
		dbPath:           dbPath,
		db:               nil,
		allowedAddresses: allowedAddressesMap,
	}
}

func (s *server) Start() error {
	return s.CtxStart(
		// on startup,
		func() error {
			// s.SetLogLabel(s.address.Pretty() + " store:badger")
			s.Infof(0, "opening badger store at %v", s.dbPath)

			db, err := badger.Open(badger.DefaultOptions(s.dbPath))
			if err != nil {
				return err
			}
			s.db = db

			s.Infof(0, "opening grpc listener at %v:%v", s.listenNetwork, s.listenHost)
			lis, err := net.Listen(s.listenNetwork, s.listenHost)
			if err != nil {
				return err
			}

			// handshaker := newGrpcHandshaker(s.allowedAddresses, nil)

			var opts []grpc.ServerOption = []grpc.ServerOption{
				// StreamServerLogger(s.Ctx()),
				// UnaryServerLogger(s.Ctx()),
				// UnaryServerJWT(s.Ctx()),
				// StreamServerJWT(s.Ctx()),
				// grpc.Creds(handshaker),
			}
			s.grpc = grpc.NewServer(opts...)
			RegisterRemoteStoreServer(s.grpc, s)
			go func() { s.grpc.Serve(lis) }()

			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {
			s.db.Close()
			s.grpc.GracefulStop()
		},
	)
}

func (s *server) Close() error {
	s.CtxStop("", nil)
	return nil
}

func (s *server) requireAuth(ctx context.Context) error {
	tokenString, err := grpc_auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		return err
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, grpc.Errorf(codes.Unauthenticated, "Unexpected signing method: %v", token.Header["alg"])
		}
		return JWT_SECRET, nil
	})
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		addrStr, ok := claims["address"].(string)
		if !ok {
			return grpc.Errorf(codes.Unauthenticated, "invalid claims")
		}

		addr, err := rw.AddressFromHex(addrStr)
		if err != nil {
			return grpc.Errorf(codes.Unauthenticated, "not allowed")
		} else if !s.allowedAddresses[addr] {
			return grpc.Errorf(codes.Unauthenticated, "not allowed")
		}
		return nil

	} else {
		return grpc.Errorf(codes.Unauthenticated, "invalid auth token: %v", err)
	}
}

func (s *server) Authenticate(authSrv RemoteStore_AuthenticateServer) error {
	challenge, err := rw.GenerateChallengeMsg()
	if err != nil {
		return err
	}

	err = authSrv.Send(&AuthenticateMessage{
		Payload: &AuthenticateMessage_AuthenticateChallenge_{&AuthenticateMessage_AuthenticateChallenge{
			Challenge: challenge,
		}},
	})

	msg, err := authSrv.Recv()
	if err != nil {
		return err
	}

	sig := msg.GetAuthenticateSignature()
	if sig == nil {
		return errors.New("protocol error")
	}

	pubkey, err := rw.RecoverSigningPubkey(rw.HashBytes(challenge), sig.Signature)
	if err != nil {
		return err
	}

	if !s.allowedAddresses[pubkey.Address()] {
		return grpc.Errorf(codes.Unauthenticated, "invalid signature")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"address": pubkey.Address().String(),
	})

	// Sign and get the complete encoded token as a string using the secret
	jwt, err := token.SignedString(JWT_SECRET)
	if err != nil {
		return err
	}

	err = authSrv.Send(&AuthenticateMessage{
		Payload: &AuthenticateMessage_AuthenticateResponse_{&AuthenticateMessage_AuthenticateResponse{
			Jwt: jwt,
		}},
	})
	return err
}

func (s *server) AddTx(ctx context.Context, req *AddTxRequest) (*AddTxResponse, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, err
	}

	key := append([]byte("tx:"), req.TxHash[:]...)

	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, req.TxBytes)
	})
	if err != nil {
		s.Errorf("failed to write tx %0x", req.TxHash)
		return nil, err
	}
	return &AddTxResponse{}, nil
}

func (s *server) RemoveTx(ctx context.Context, req *RemoveTxRequest) (*RemoveTxResponse, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, err
	}

	key := append([]byte("tx:"), req.TxHash...)
	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
	if err != nil {
		return nil, err
	}
	return &RemoveTxResponse{}, nil
}

func (s *server) FetchTx(ctx context.Context, req *FetchTxRequest) (*FetchTxResponse, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, err
	}

	key := append([]byte("tx:"), req.TxHash...)

	var txBytes []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			txBytes = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return &FetchTxResponse{TxBytes: txBytes}, err
}

func (s *server) AllTxs(req *AllTxsRequest, server RemoteStore_AllTxsServer) error {
	if err := s.requireAuth(server.Context()); err != nil {
		return err
	}

	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		badgerIter := txn.NewIterator(opts)
		defer badgerIter.Close()

		prefix := []byte("tx:")
		for badgerIter.Seek(prefix); badgerIter.ValidForPrefix(prefix); badgerIter.Next() {
			item := badgerIter.Item()

			var txBytes []byte
			err := item.Value(func(val []byte) error {
				txBytes = append([]byte{}, val...)
				return nil
			})
			if err != nil {
				return err
			}

			err = server.Send(&AllTxsResponsePacket{TxBytes: txBytes})
			if err != nil {
				return err
			}

			select {
			case <-server.Context().Done():
				return server.Context().Err()
			default:
			}
		}
		return nil
	})
}
