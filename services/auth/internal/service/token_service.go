package service

import (
	"crypto/rsa"
	"encoding/hex"
	"strings"
	"time"

	"crypto/rand"
	"crypto/sha256"

	"github.com/golang-jwt/jwt/v5"
)

type ITokenService interface {
	GenerateAccessToken(userId, role string) (string, error)
	GenerateRefreshToken() (string, error)
	HashToken(token string) string
}

type TokenService struct {
	privateKey *rsa.PrivateKey
}

func NewTokenService(privateKeyPEM string) (*TokenService, error) {
	privateKeyPEM = strings.ReplaceAll(privateKeyPEM, `\n`, "\n")

	parsed, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return nil, err
	}

	return &TokenService{
		privateKey: parsed,
	}, nil
}

func (s *TokenService) GenerateAccessToken(userId, role string) (string, error) {
	accessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"user_id": userId,
		"role":    role,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
	})

	accessTokenString, err := accessToken.SignedString(s.privateKey)
	if err != nil {
		return "", err
	}

	return accessTokenString, nil
}

func (s *TokenService) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)

	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	refreshToken := hex.EncodeToString(b)

	return refreshToken, nil
}

func (s *TokenService) HashToken(token string) string {
	hash := sha256.New()
	hash.Write([]byte(token))

	hashedToken := hex.EncodeToString(hash.Sum(nil))
	return hashedToken
}
