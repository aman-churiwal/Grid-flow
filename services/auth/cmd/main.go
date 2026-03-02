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

	"github.com/aman-churiwal/gridflow-auth/internal/repository"
	"github.com/aman-churiwal/gridflow-shared/config"
	"github.com/aman-churiwal/gridflow-shared/logger"
	_ "github.com/lib/pq"
)

func main() {
	c, err := config.Load()
	if err != nil {
		fmt.Printf("Unable to load config: %v", err)
		return
	}

	appLogger := logger.NewLogger(c.ServiceName, "INFO", c.AppEnv)

	db, err := repository.NewPostgres(c.PostgresDSN)
	if err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("Unable to connect to database")
		return
	}

	if err := db.Ping(); err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("Unable to reach database")
		return
	}

	if err := repository.RunMigrations(c.PostgresDSN, "migrations"); err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("Failed to run migrations")
		return
	}
	appLogger.Info(context.Background()).Msg("Migrations applied successfully")

	router := newRouter(appLogger)

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

	if err := db.Close(); err != nil {
		appLogger.Error(context.Background()).Err(err).Msg("Unable to close database connection")
	}

	appLogger.Info(context.Background()).Msg("Server Exiting")
}
