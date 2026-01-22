package core

import (
	"fmt"
	"goonhub/internal/data"
	"time"

	"github.com/o1egl/paseto"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo      data.UserRepository
	pasetoKey []byte
	tokenTTL  time.Duration
	logger    *zap.Logger
	v2        *paseto.V2
}

type UserPayload struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func NewAuthService(repo data.UserRepository, pasetoSecret string, tokenTTL time.Duration, logger *zap.Logger) *AuthService {
	return &AuthService{
		repo:      repo,
		pasetoKey: []byte(pasetoSecret),
		tokenTTL:  tokenTTL,
		logger:    logger,
		v2:        paseto.NewV2(),
	}
}

func (s *AuthService) Login(username, password string) (string, *data.User, error) {
	user, err := s.repo.GetByUsername(username)
	if err != nil {
		s.logger.Error("User not found", zap.String("username", username))
		return "", nil, fmt.Errorf("invalid credentials")
	}

	if err := s.checkPassword(user.Password, password); err != nil {
		s.logger.Error("Invalid password", zap.String("username", username))
		return "", nil, fmt.Errorf("invalid credentials")
	}

	token, err := s.generateToken(user)
	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		return "", nil, fmt.Errorf("failed to generate token")
	}

	s.logger.Info("User logged in", zap.String("username", username), zap.Uint("user_id", user.ID))
	return token, user, nil
}

func (s *AuthService) ValidateToken(token string) (*UserPayload, error) {
	var payload UserPayload

	err := s.v2.Decrypt(token, s.pasetoKey, &payload, nil)
	if err != nil {
		s.logger.Error("Invalid token", zap.Error(err))
		return nil, fmt.Errorf("invalid token")
	}

	now := time.Now()
	if now.Unix() > payload.ExpiresAt {
		s.logger.Warn("Token expired", zap.Int64("expired_at", payload.ExpiresAt), zap.Int64("current", now.Unix()))
		return nil, fmt.Errorf("token expired")
	}

	return &payload, nil
}

func (s *AuthService) hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashedPassword), nil
}

func (s *AuthService) checkPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func (s *AuthService) generateToken(user *data.User) (string, error) {
	now := time.Now()
	payload := UserPayload{
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.tokenTTL).Unix(),
	}

	token, err := s.v2.Encrypt(s.pasetoKey, payload, nil)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt token: %w", err)
	}

	return token, nil
}
