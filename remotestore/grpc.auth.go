package remotestore

// import (
// 	"context"
// 	"net"
// 	"net/http"

// 	"google.golang.org/grpc"
// 	"google.golang.org/grpc/codes"
// 	"google.golang.org/grpc/credentials"

// 	rw "redwood.dev"
// )

// // grpcHandshaker is the credentials required for authenticating a connection using ALTS.
// // It implements credentials.TransportCredentials interface.
// type grpcHandshaker struct {
// 	info             *credentials.ProtocolInfo
// 	allowedAddresses map[rw.Address]bool
// 	clientSigprivkey rw.SigningPrivateKey
// }

// type authError struct {
// 	msg       string
// 	temporary bool
// }

// func (e *authError) Error() string {
// 	return e.msg
// }
// func (e *authError) Temporary() bool {
// 	return e.temporary
// }

// func newGrpcHandshaker(allowedAddresses []rw.Address, clientSigprivkey rw.SigningPrivateKey) credentials.TransportCredentials {
// 	allowedAddressesMap := make(map[rw.Address]bool)
// 	for _, addr := range allowedAddresses {
// 		allowedAddressesMap[addr] = true
// 	}

// 	return &grpcHandshaker{
// 		info: &credentials.ProtocolInfo{
// 			SecurityProtocol: "redwood",
// 			SecurityVersion:  "1.0",
// 		},
// 		allowedAddresses: allowedAddressesMap,
// 		clientSigprivkey: clientSigprivkey,
// 	}
// }

// // ClientHandshake implements the client side handshake protocol.
// func (g *grpcHandshaker) ClientHandshake(ctx context.Context, addr string, rawConn net.Conn) (_ net.Conn, _ credentials.AuthInfo, err error) {
// 	defer func() {
// 		if err != nil {
// 			rawConn.Close()
// 		}
// 	}()

// 	// Possible context leak:
// 	// The cancel function for the child context we create will only be
// 	// called a non-nil error is returned.
// 	ctx, cancel := context.WithCancel(ctx)
// 	// defer func() {
// 	// 	if err != nil {
// 	// 		cancel()
// 	// 	}
// 	// }()
// 	defer cancel()

// 	var msg rw.Msg
// 	err = rw.ReadMsg(rawConn, &msg)
// 	if err != nil {
// 		return nil, nil, &authError{err.Error(), true}
// 	}

// 	if msg.Type != rw.MsgType_VerifyAddress {
// 		return nil, nil, &authError{"grpc auth protocol error", false}
// 	}
// 	challengeMsg, is := msg.Payload.([]byte)
// 	if !is {
// 		return nil, nil, &authError{"grpc auth protocol error", false}
// 	}

// 	sig, err := g.clientSigprivkey.SignHash(rw.HashBytes(challengeMsg))
// 	if err != nil {
// 		return nil, nil, &authError{err.Error(), false}
// 	}

// 	err = rw.WriteMsg(rawConn, rw.Msg{Type: rw.MsgType_VerifyAddressResponse, Payload: rw.VerifyAddressResponse{
// 		Signature: sig,
// 	}})
// 	if err != nil {
// 		return nil, nil, &authError{err.Error(), false}
// 	}

// 	statusCode, err := rw.ReadUint64(rawConn)
// 	if err != nil {
// 		return nil, nil, &authError{err.Error(), true}
// 	} else if statusCode != http.StatusOK {
// 		// return nil, nil, &authError{"not allowed", false}
// 		return nil, nil, grpc.Errorf(codes.Unauthenticated, "not allowed")
// 	}
// 	return rawConn, authInfo{}, nil
// }

// func (g *grpcHandshaker) ServerHandshake(rawConn net.Conn) (_ net.Conn, _ credentials.AuthInfo, err error) {
// 	defer func() {
// 		if err != nil {
// 			rawConn.Close()
// 		}
// 	}()

// 	challengeMsg, err := rw.GenerateChallengeMsg()
// 	if err != nil {
// 		return nil, nil, &authError{err.Error(), false}
// 	}

// 	err = rw.WriteMsg(rawConn, rw.Msg{Type: rw.MsgType_VerifyAddress, Payload: challengeMsg})
// 	if err != nil {
// 		return nil, nil, &authError{err.Error(), true}
// 	}

// 	var msg rw.Msg
// 	err = rw.ReadMsg(rawConn, &msg)
// 	if err != nil {
// 		return nil, nil, &authError{err.Error(), true}
// 	}

// 	if msg.Type != rw.MsgType_VerifyAddressResponse {
// 		return nil, nil, &authError{"grpc auth protocol error", false}
// 	}
// 	resp, is := msg.Payload.(rw.VerifyAddressResponse)
// 	if !is {
// 		return nil, nil, &authError{"grpc auth protocol error", false}
// 	}

// 	pubkey, err := rw.RecoverSigningPubkey(rw.HashBytes(challengeMsg), resp.Signature)
// 	if !is {
// 		return nil, nil, &authError{"bad signature", false}
// 	}

// 	if !g.allowedAddresses[pubkey.Address()] {
// 		err := rw.WriteUint64(rawConn, http.StatusForbidden)
// 		if err != nil {
// 			return nil, nil, &authError{err.Error(), false}
// 		}
// 		return nil, nil, &authError{"not allowed", false}
// 	}

// 	err = rw.WriteUint64(rawConn, http.StatusOK)
// 	if err != nil {
// 		return nil, nil, &authError{err.Error(), true}
// 	}
// 	return rawConn, authInfo{}, nil
// }

// func (g *grpcHandshaker) Info() credentials.ProtocolInfo {
// 	return *g.info
// }

// func (g *grpcHandshaker) Clone() credentials.TransportCredentials {
// 	info := *g.info
// 	allowedAddresses := make(map[rw.Address]bool)
// 	for k := range g.allowedAddresses {
// 		allowedAddresses[k] = true
// 	}
// 	return &grpcHandshaker{
// 		info:             &info,
// 		allowedAddresses: allowedAddresses,
// 	}
// }

// func (g *grpcHandshaker) OverrideServerName(serverNameOverride string) error {
// 	g.info.ServerName = serverNameOverride
// 	return nil
// }

// type authInfo struct{}

// func (i authInfo) AuthType() string {
// 	return "redwood"
// }
