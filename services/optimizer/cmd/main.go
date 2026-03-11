package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aman-churiwal/gridflow-shared/cache"
	"github.com/aman-churiwal/gridflow-shared/config"
	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/proto/gen"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/kafka-go"
)

func main() {
	c, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to load config: %v\n", err)
		return
	}

	appLogger := logger.NewLogger(c.ServiceName, "INFO", c.AppEnv)

	if c.HealthPort == 0 {
		appLogger.Error(context.Background()).Msg("HEALTH_PORT is required")
		return
	}

	if c.Topic == "" {
		appLogger.Error(context.Background()).Msg("TOPIC is required")
		return
	}

	if c.GroupID == "" {
		appLogger.Error(context.Background()).Msg("GROUP_ID is required")
		return
	}

	redisClient := cache.NewRedisClient(c.RedisAddr)
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("unable to connect to Redis")
		return
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        c.KafkaBrokers,
		Topic:          c.Topic,
		GroupID:        c.GroupID,
		MinBytes:       100,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
	})

	router := gin.New()

	healthAddr := fmt.Sprintf(":%d", c.HealthPort)
	srv := &http.Server{
		Addr:    healthAddr,
		Handler: router.Handler(),
	}
	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": c.ServiceName,
		})
	})

	go func(logger *logger.Logger) {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Error(context.Background()).Err(err).Msg("Error listening to server")
		}
	}(appLogger)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			m, err := reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				appLogger.Error(ctx).Err(err).Msg("error reading message")
				continue
			}

			var ping gen.VehiclePing
			if err := json.Unmarshal(m.Value, &ping); err != nil {
				appLogger.Error(ctx).Err(err).Msg("error unmarshalling message")
				continue
			}
			appLogger.Info(ctx).
				Str("vehicle_id", ping.VehicleId).
				Str("topic", m.Topic).
				Msg("message received")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appLogger.Info(context.Background()).Msg("Shutting down server...")
	cancel()

	if err := reader.Close(); err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("Unable to close Kafka reader")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("Server shutdown failed")
	}

	appLogger.Info(context.Background()).Msg("Server Exiting")
}
