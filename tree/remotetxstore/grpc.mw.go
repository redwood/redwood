package remotetxstore

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	ctxpkg "redwood.dev/log"
)

func UnaryServerLogger(log ctxpkg.Logger) grpc.ServerOption {
	return grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		log.Infof(0, "%v %+v", info.FullMethod, req)

		x, err := handler(ctx, req)
		if err != nil {
			log.Errorf("%v %+v %+v", info.FullMethod, req, err)
		}

		log.Infof(0, "%v %+v, %+v", info.FullMethod, req, x)
		return x, err
	})
}

func StreamServerLogger(log ctxpkg.Logger) grpc.ServerOption {
	return grpc.StreamInterceptor(func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		log.Infof(0, "%v", info.FullMethod)
		err := handler(srv, stream)
		if err != nil {
			log.Errorf("%+v", err)
		}
		return err
	})
}

var JWT_SECRET = []byte("jwt secret string") // @@TODO: make configurable

func UnaryClientJWT(redwoodClient *client) grpc.DialOption {
	return grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if redwoodClient.jwt != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "bearer "+redwoodClient.jwt)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	})
}

func StreamClientJWT(redwoodClient *client) grpc.DialOption {
	return grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if redwoodClient.jwt != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "bearer "+redwoodClient.jwt)
		}
		return streamer(ctx, desc, cc, method, opts...)
	})
}
