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

	"github.com/aman-churiwal/gridflow-shared/cache"
	"github.com/aman-churiwal/gridflow-shared/config"
	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/gin-gonic/gin"
)

func main() {
	c, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to load config: %v", err)
		return
	}

	appLogger := logger.NewLogger(c.ServiceName, "INFO", c.AppEnv)

	redisClient := cache.NewRedisClient(c.RedisAddr)

	router := gin.New()
	router.POST("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "optimizer",
		})
	})

	addr := fmt.Sprintf(":%d", c.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router.Handler(),
	}

	go func(log *logger.Logger) {
		log.Info(context.Background()).Str("addr", addr).Msg("Server starting...")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error(context.Background()).Err(err).Msg("Error listening to server")
		}
	}(appLogger)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appLogger.Info(context.Background()).Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("Server shutdown failed")
	}

	if err := redisClient.Close(); err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("Redis shutdown failed")
	}
}
