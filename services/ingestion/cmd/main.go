package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aman-churiwal/gridflow-ingestion/internal/server"
	"github.com/aman-churiwal/gridflow-ingestion/internal/session"
	"github.com/aman-churiwal/gridflow-shared/config"
	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	c, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to load config: %v", err)
		return
	}

	appLogger := logger.NewLogger(c.ServiceName, "INFO", c.AppEnv)

	// initialize grpc server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Port))
	if err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("error listen to port")
		return
	}

	sessionStore := session.NewSessionStore()
	ingestionServer := server.NewIngestionServer(sessionStore, appLogger)

	grpcServer := grpc.NewServer()
	gen.RegisterIngestionServiceServer(grpcServer, ingestionServer)
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	appLogger.Info(context.Background()).Msg("gRPC server starting...")
	// start listening
	go func() {
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			appLogger.Error(context.Background()).Err(err).Msg("error serving grpc")
			return
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appLogger.Info(context.Background()).Msg("Shutting down server...")

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		appLogger.Info(context.Background()).Msg("server stopped gracefully")
	case <-time.After(5 * time.Second):
		appLogger.Warn(context.Background()).Msg("Graceful shutdown timed out, forcing stop")
		grpcServer.Stop()
	}
}
