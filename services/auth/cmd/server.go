package main

import (
	"database/sql"

	"github.com/aman-churiwal/gridflow-auth/internal/handler"
	"github.com/aman-churiwal/gridflow-auth/internal/repository"
	"github.com/aman-churiwal/gridflow-auth/internal/service"
	"github.com/aman-churiwal/gridflow-shared/config"
	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/middleware"
	"github.com/gin-gonic/gin"
)

func newRouter(appLogger *logger.Logger, db *sql.DB, cfg config.Config) (*gin.Engine, error) {
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	tokenService, err := service.NewTokenService(cfg.JwtPrivateKey)
	if err != nil {
		return nil, err
	}
	authService := service.NewAuthService(userRepo, tokenRepo, tokenService)
	authHandler := handler.NewAuthHandler(authService)

	router := gin.New()

	router.Use(middleware.RequestLogger(appLogger))

	router.GET("/health", handler.Health)

	router.POST("/auth/register", authHandler.Register)
	router.POST("/auth/login", authHandler.Login)
	router.POST("/auth/refresh", authHandler.RefreshToken)

	protected := router.Group("/")
	protected.Use(middleware.JWTMiddleware(cfg.JwtPublicKey))
	return router, nil
}
