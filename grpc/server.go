package grpc

import (
	"crypto/tls"
	"net/http"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"golang.org/x/net/context"
)

type server struct {
	backendAddr       string
	next              httpserver.Handler
	backendIsInsecure bool
	backendTLS        *tls.Config
	wrappedGrpc       *grpcweb.WrappedGrpcServer
}

// ServeHTTP satisfies the httpserver.Handler interface.
func (s server) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	//dial Backend
	opt := []grpc.DialOption{}
	opt = append(opt, grpc.WithCodec(proxy.Codec()))
	if s.backendIsInsecure {
		opt = append(opt, grpc.WithInsecure())
	} else {
		opt = append(opt, grpc.WithTransportCredentials(credentials.NewTLS(s.backendTLS)))
	}

	backendConn, err := grpc.Dial(s.backendAddr, opt...)
	if err != nil {
		return s.next.ServeHTTP(w, r)
	}

	director := func(ctx context.Context, fullMethodName string) (*grpc.ClientConn, error) {
		return backendConn, nil
	}
	grpcServer := grpc.NewServer(
		grpc.CustomCodec(proxy.Codec()), // needed for proxy to function.
		grpc.UnknownServiceHandler(proxy.TransparentHandler(director)),
		/*grpc_middleware.WithUnaryServerChain(
			grpc_logrus.UnaryServerInterceptor(logger),
			grpc_prometheus.UnaryServerInterceptor,
		),
		grpc_middleware.WithStreamServerChain(
			grpc_logrus.StreamServerInterceptor(logger),
			grpc_prometheus.StreamServerInterceptor,
		),*/ //middleware should be a config setting or 3rd party middleware plugins like for caddyhttp
	)

	// gRPC-Web compatibility layer with CORS configured to accept on every
	wrappedGrpc := grpcweb.WrapServer(grpcServer, grpcweb.WithCorsForRegisteredEndpointsOnly(false))
	wrappedGrpc.ServeHTTP(w, r)

	return 0, nil
}

func (s server) Stop() {
	// TODO(pieterlouw): Add graceful shutdown.
	// Currently grpcweb.WrappedGrpcServer don't have a Stop/GracefulStop method
}
