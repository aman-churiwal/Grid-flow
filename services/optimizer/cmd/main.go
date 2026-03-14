package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aman-churiwal/gridflow-optimizer/internal/consumer"
	"github.com/aman-churiwal/gridflow-optimizer/internal/lock"
	"github.com/aman-churiwal/gridflow-optimizer/internal/optimizer"
	"github.com/aman-churiwal/gridflow-shared/cache"
	"github.com/aman-churiwal/gridflow-shared/config"
	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/kafka-go"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	c, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to load config: %v\n", err)
		return
	}

	if c.GeoRadiusKm == 0 {
		c.GeoRadiusKm = 5
	}

	appLogger := logger.NewLogger(c.ServiceName, "INFO", c.AppEnv)

	if err := validateConfig(c); err != nil {
		appLogger.Error(context.Background()).Err(err).Msg(err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	redisClient := cache.NewRedisClient(c.RedisAddr)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		appLogger.Error(ctx).Err(err).Msg("unable to connect to Redis")
		return
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  c.KafkaBrokers,
		Topic:    c.Topic,
		GroupID:  c.GroupID,
		MinBytes: 100,
		MaxBytes: 10e6,
	})
	msgs := make(chan consumer.Message, 100)
	consumerGroup := consumer.NewConsumerGroup(reader, msgs, appLogger)
	geoStore := optimizer.NewGeoStore(redisClient, appLogger, c.GeoRadiusKm)
	newOptimizer := optimizer.NewOptimizer(msgs, geoStore, appLogger)

	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   c.EtcdEndpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		appLogger.Error(ctx).Err(err).Msg("unable to connect to etcd")
		return
	}

	election, err := lock.NewElection(etcdClient, appLogger)
	if err != nil {
		appLogger.Error(ctx).Err(err).Msg("unable to create election")
		return
	}

	consumerGroup.Start(ctx)
	go lock.LeaderLoop(ctx, election, newOptimizer, appLogger)

	srv := newHealthServer(c.HealthPort, c.ServiceName)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Error(ctx).Err(err).Msg("Error listening to server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appLogger.Info(context.Background()).Msg("Shutting down server...")

	// Stopping consumer and optimizer
	cancel()

	// Releasing leadership
	_ = election.Resign(context.Background())
	_ = election.Close()
	_ = etcdClient.Close()

	// Draining kafka
	_ = consumerGroup.Close()

	// Stopping health server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)

	appLogger.Info(context.Background()).Msg("Server Exiting")
}

func validateConfig(c config.Config) error {
	if c.HealthPort == 0 {
		return errors.New("HEALTH_PORT is required")
	}

	if c.Topic == "" {
		return errors.New("TOPIC is required")
	}

	if c.GroupID == "" {
		return errors.New("GROUP_ID is required")
	}

	return nil
}

func newHealthServer(port int, serviceName string) *http.Server {
	router := gin.New()

	healthAddr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:    healthAddr,
		Handler: router.Handler(),
	}
	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": serviceName,
		})
	})

	return srv
}
