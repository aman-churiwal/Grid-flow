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

	"github.com/aman-churiwal/gridflow-ingestion/internal/publisher"
	"github.com/aman-churiwal/gridflow-ingestion/internal/ratelimit"
	"github.com/aman-churiwal/gridflow-ingestion/internal/server"
	"github.com/aman-churiwal/gridflow-ingestion/internal/session"
	"github.com/aman-churiwal/gridflow-ingestion/internal/worker"
	"github.com/aman-churiwal/gridflow-shared/cache"
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

	if c.WorkerPoolSize == 0 {
		c.WorkerPoolSize = 10
	}
	if c.RateLimitMaxPings == 0 {
		c.RateLimitMaxPings = 30
	}
	if c.RateLimitWindowSecs == 0 {
		c.RateLimitWindowSecs = 60
	}

	appLogger := logger.NewLogger(c.ServiceName, "INFO", c.AppEnv)

	// initialize grpc server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Port))
	if err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("error listen to port")
		return
	}

	sessionStore := session.NewSessionStore()
	client := cache.NewRedisClient(c.RedisAddr)
	rateLimiter := ratelimit.NewRateLimiter(client, c.RateLimitMaxPings, c.RateLimitWindowSecs)
	ingestionServer := server.NewIngestionServer(sessionStore, appLogger, rateLimiter)

	kafkaPublisher := publisher.NewKafkaPublisher(c.KafkaBrokers, "vehicle.telemetry", appLogger)
	workerPool := worker.NewPool(c.WorkerPoolSize, ingestionServer.Pings(), appLogger, kafkaPublisher)
	ctx, cancel := context.WithCancel(context.Background())
	workerPool.Start(ctx)

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
	cancel()

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

	workerPool.Wait()
	if err := kafkaPublisher.Close(); err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("error closing kafka publisher")
		return
	}
}
