package middleware

import (
	"time"

	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/gin-gonic/gin"
)

func RequestLogger(appLogger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		appLogger.Info(c.Request.Context()).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("duration", time.Since(start)).
			Msg("Request completed")
	}
}
