package grapiserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// NewGatewayServer creates GrpcServer instance.
func NewGatewayServer(c *Config) Server {
	return &GatewayServer{
		Config: c,
	}
}

// GatewayServer wraps gRPC gateway server setup process.
type GatewayServer struct {
	server *http.Server
	*Config
}

// Serve implements Server.Shutdown
func (s *GatewayServer) Serve(l net.Listener, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := s.createConn()
	if err != nil {
		s.Logger.Error("failed to create connection with gRPC server", LogFields{"error": err})
		return
	}
	defer conn.Close()

	s.server, err = s.createServer(conn)
	if err != nil {
		s.Logger.Error("failed to create gRPC Gateway server", LogFields{"error": err})
		return
	}

	s.Logger.Info("gRPC Gateway server is starting", LogFields{"network": s.GatewayAddr.Network, "addr": s.GatewayAddr.Addr})
	err = s.server.Serve(l)
	s.Logger.Info("Stopped taking more httr(s) requests", LogFields{"error": err})
}

// Shutdown implements Server.Shutdown
func (s *GatewayServer) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := s.server.Shutdown(ctx)
	s.Logger.Info("All http(s) requets finished", LogFields{})
	if err != nil {
		s.Logger.Error("Failed to shutdown gRPC Gateway server", LogFields{"error": err})
	}
}

func (s *GatewayServer) createConn() (conn *grpc.ClientConn, err error) {
	conn, err = grpc.Dial(s.GrpcInternalAddr.Addr, s.clientOptions()...)
	if err != nil {
		err = errors.Wrap(err, "failed to connect to gRPC server")
	}
	return
}

func (s *GatewayServer) createServer(conn *grpc.ClientConn) (*http.Server, error) {
	mux := runtime.NewServeMux(
		append(
			s.GatewayMuxOptions,
			runtime.WithProtoErrorHandler(runtime.DefaultHTTPProtoErrorHandler),
		)...,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	s.Logger.Info(fmt.Sprintf("Register %d server handlers to gRPC Gateway", len(s.RegisterGatewayHandlerFuncs)), LogFields{})
	for _, register := range s.RegisterGatewayHandlerFuncs {
		err := register(ctx, mux, conn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to register handler")
		}
	}

	var handler http.Handler = mux

	if s.HTTPHeaderMappingConfig != nil {
		mapper := newHTTPHeaderMapper(s.HTTPHeaderMappingConfig)
		handler = mapper.wrap(handler)
	}

	return &http.Server{
		ReadTimeout:  8 * time.Second,
		WriteTimeout: 8 * time.Second,
		IdleTimeout:  2 * time.Minute,
		Handler:      handler,
	}, nil
}
