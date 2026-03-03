package service

import (
	"context"
	"errors"
	"time"

	"github.com/aman-churiwal/gridflow-auth/internal/model"
	"github.com/aman-churiwal/gridflow-auth/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrRefreshTokenInvalid = errors.New("refresh token is invalid")
	ErrRefreshTokenExpired = errors.New("refresh token has expired")
)

const refreshTokenExpiry = 7 * 24 * time.Hour

type IAuthService interface {
	Register(ctx context.Context, req *model.RegisterRequest) (*model.RegisterResponse, error)
	Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error)
	Refresh(ctx context.Context, req *model.RefreshRequest) (*model.RefreshResponse, error)
}

type AuthService struct {
	authRepository  repository.IUserRepository
	tokenRepository repository.ITokenRepository
	tokenService    ITokenService
}

func NewAuthService(authRepository repository.IUserRepository, tokenRepository repository.ITokenRepository, tokenService ITokenService) *AuthService {
	return &AuthService{
		authRepository:  authRepository,
		tokenRepository: tokenRepository,
		tokenService:    tokenService,
	}
}

func (s *AuthService) Register(ctx context.Context, req *model.RegisterRequest) (*model.RegisterResponse, error) {
	userExists, err := s.authRepository.UserExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	if userExists {
		return nil, ErrEmailAlreadyExists
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, err
	}

	newUserID, err := s.authRepository.CreateUser(ctx, req.Email, string(passwordHash))
	if err != nil {
		return nil, err
	}

	return &model.RegisterResponse{
		ID:    newUserID,
		Email: req.Email,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error) {
	user, err := s.authRepository.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	accessToken, err := s.tokenService.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return nil, err
	}

	rawRefreshToken, err := s.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	hashedRefreshToken := s.tokenService.HashToken(rawRefreshToken)

	if err := s.tokenRepository.StoreRefreshToken(ctx, user.ID, hashedRefreshToken, time.Now().Add(refreshTokenExpiry)); err != nil {
		return nil, err
	}

	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, req *model.RefreshRequest) (*model.RefreshResponse, error) {
	hashedToken := s.tokenService.HashToken(req.RefreshToken)
	refreshToken, err := s.tokenRepository.GetRefreshToken(ctx, hashedToken)
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenNotFound) {
			return nil, ErrRefreshTokenInvalid
		}
		return nil, err
	}

	if refreshToken.Revoked {
		return nil, ErrRefreshTokenInvalid
	}

	if time.Now().After(refreshToken.ExpiresAt) {
		return nil, ErrRefreshTokenExpired
	}

	if err := s.tokenRepository.RevokeRefreshToken(ctx, refreshToken.ID); err != nil {
		return nil, err
	}

	user, err := s.authRepository.GetUserByID(ctx, refreshToken.UserID)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.tokenService.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	hashedRefreshToken := s.tokenService.HashToken(newRefreshToken)
	if err := s.tokenRepository.StoreRefreshToken(ctx, user.ID, hashedRefreshToken, time.Now().Add(refreshTokenExpiry)); err != nil {
		return nil, err
	}

	return &model.RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}
