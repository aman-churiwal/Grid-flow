package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aman-churiwal/gridflow-shared/cache"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	ContextKeyUserID contextKey = "user_id"
	ContextKeyRole   contextKey = "role"
)

func JWTMiddleware(publicKeyPEM string, jwtCache cache.IJwtCache) gin.HandlerFunc {
	publicKeyPEM = strings.ReplaceAll(publicKeyPEM, `\n`, "\n")
	parsedPublicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicKeyPEM))
	if err != nil {
		panic("invalid RSA public key: " + err.Error())
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authorization header not found",
			})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format, use 'Bearer <token>'",
			})
			return
		}

		tokenString := parts[1]

		tokenHash := hashToken(tokenString)
		if jwtCache != nil {
			cached, err := jwtCache.Get(c.Request.Context(), tokenHash)
			if err == nil && cached != nil {
				// Cache hit
				ctx := context.WithValue(c.Request.Context(), ContextKeyUserID, cached.UserID)
				ctx = context.WithValue(ctx, ContextKeyRole, cached.Role)
				c.Request = c.Request.WithContext(ctx)
				c.Next()
				return
			}
		}

		token, err := jwt.ParseWithClaims(tokenString, &jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}

			return parsedPublicKey, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(*jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		userID, ok := (*claims)["user_id"].(string)
		if !ok || userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		role, ok := (*claims)["role"].(string)
		if !ok || role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), ContextKeyUserID, userID)
		ctx = context.WithValue(ctx, ContextKeyRole, role)
		c.Request = c.Request.WithContext(ctx)

		if jwtCache != nil {
			exp, _ := claims.GetExpirationTime()
			if exp != nil {
				ttl := time.Until(exp.Time)
				if ttl > 0 {
					_ = jwtCache.Set(ctx, tokenHash, &cache.CachedClaims{
						UserID: userID,
						Role:   role,
					}, ttl)
				}
			}
		}

		c.Next()
	}
}

func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
	return userID, ok
}

func GetRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(ContextKeyRole).(string)
	return role, ok
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
