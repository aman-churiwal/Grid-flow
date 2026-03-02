package main

import (
	"github.com/aman-churiwal/gridflow-auth/internal/handler"
	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/middleware"
	"github.com/gin-gonic/gin"
)

func newRouter(appLogger *logger.Logger) *gin.Engine {
	router := gin.New()

	router.Use(middleware.RequestLogger(appLogger))

	router.GET("/health", handler.Health)

	return router
}
