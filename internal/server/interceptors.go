package server

import (
	"context"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// loggingInterceptor logs unary RPC calls
func (s *Server) loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	duration := time.Since(start)

	if err != nil {
		s.logger.Error("unary RPC error",
			"method", info.FullMethod,
			"duration", duration,
			"error", err,
		)
	} else {
		s.logger.Info("unary RPC",
			"method", info.FullMethod,
			"duration", duration,
		)
	}

	return resp, err
}

// recoveryInterceptor recovers from panics in unary RPC handlers
func (s *Server) recoveryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic recovered in unary RPC",
				"method", info.FullMethod,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			err = status.Errorf(codes.Internal, "internal server error")
		}
	}()

	return handler(ctx, req)
}

// streamLoggingInterceptor logs streaming RPC calls
func (s *Server) streamLoggingInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()

	err := handler(srv, ss)

	duration := time.Since(start)

	if err != nil {
		s.logger.Error("stream RPC error",
			"method", info.FullMethod,
			"duration", duration,
			"error", err,
		)
	} else {
		s.logger.Info("stream RPC",
			"method", info.FullMethod,
			"duration", duration,
		)
	}

	return err
}

// streamRecoveryInterceptor recovers from panics in streaming RPC handlers
func (s *Server) streamRecoveryInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic recovered in stream RPC",
				"method", info.FullMethod,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			err = status.Errorf(codes.Internal, "internal server error")
		}
	}()

	return handler(srv, ss)
}
