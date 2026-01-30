package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"goonhub/internal/data"
	"sync"
	"time"

	"github.com/o1egl/paseto"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// loginAttempt tracks failed login attempts for a username
type loginAttempt struct {
	count    int
	lockedAt time.Time
}

// AccountLockout manages per-user failed login tracking
type AccountLockout struct {
	mu        sync.RWMutex
	attempts  map[string]*loginAttempt
	threshold int
	duration  time.Duration
}

// NewAccountLockout creates a new account lockout tracker
func NewAccountLockout(threshold int, duration time.Duration) *AccountLockout {
	return &AccountLockout{
		attempts:  make(map[string]*loginAttempt),
		threshold: threshold,
		duration:  duration,
	}
}

// IsLocked checks if the username is currently locked out
func (l *AccountLockout) IsLocked(username string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	attempt, exists := l.attempts[username]
	if !exists {
		return false
	}

	if attempt.count < l.threshold {
		return false
	}

	// Check if lockout has expired
	if time.Since(attempt.lockedAt) > l.duration {
		return false
	}

	return true
}

// RecordFailure records a failed login attempt, returns true if account becomes locked
func (l *AccountLockout) RecordFailure(username string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, exists := l.attempts[username]
	if !exists {
		attempt = &loginAttempt{}
		l.attempts[username] = attempt
	}

	// If lockout expired, reset the counter
	if attempt.count >= l.threshold && time.Since(attempt.lockedAt) > l.duration {
		attempt.count = 0
	}

	attempt.count++

	if attempt.count >= l.threshold {
		attempt.lockedAt = time.Now()
		return true
	}

	return false
}

// RecordSuccess clears failed attempts on successful login
func (l *AccountLockout) RecordSuccess(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, username)
}

// Cleanup removes expired lockout entries
func (l *AccountLockout) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for username, attempt := range l.attempts {
		// Remove entries that have expired their lockout and have been idle
		if attempt.count >= l.threshold && now.Sub(attempt.lockedAt) > l.duration*2 {
			delete(l.attempts, username)
		} else if attempt.count < l.threshold && now.Sub(attempt.lockedAt) > l.duration {
			// Remove entries with few attempts that are old
			delete(l.attempts, username)
		}
	}
}

// GetRemainingLockoutTime returns how long until the lockout expires
func (l *AccountLockout) GetRemainingLockoutTime(username string) time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()

	attempt, exists := l.attempts[username]
	if !exists || attempt.count < l.threshold {
		return 0
	}

	remaining := l.duration - time.Since(attempt.lockedAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

type AuthService struct {
	repo        data.UserRepository
	revokedRepo data.RevokedTokenRepository
	pasetoKey   []byte
	tokenTTL    time.Duration
	logger      *zap.Logger
	v2          *paseto.V2
	lockout     *AccountLockout
}

type UserPayload struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// ErrPasetoKeyTooShort is returned when the PASETO secret is less than 32 bytes
var ErrPasetoKeyTooShort = fmt.Errorf("PASETO secret must be at least 32 bytes (or 64 hex characters)")

func NewAuthService(repo data.UserRepository, revokedRepo data.RevokedTokenRepository, pasetoSecret string, tokenTTL time.Duration, lockoutThreshold int, lockoutDuration time.Duration, logger *zap.Logger) (*AuthService, error) {
	// PASETO v2 requires exactly 32 bytes for the symmetric key.
	// The secret may be:
	// - A 64-character hex string (32 bytes hex-encoded) - decode it
	// - A 32-byte raw string - use directly
	// SECURITY: Reject secrets shorter than 32 bytes to prevent weak key usage
	var key []byte
	if len(pasetoSecret) == 64 {
		// Assume hex-encoded, try to decode
		decoded, err := hex.DecodeString(pasetoSecret)
		if err == nil && len(decoded) == 32 {
			key = decoded
		} else {
			// Not valid hex, use first 32 bytes
			key = []byte(pasetoSecret)[:32]
		}
	} else if len(pasetoSecret) >= 32 {
		// Use first 32 bytes
		key = []byte(pasetoSecret)[:32]
	} else {
		// SECURITY: Reject short secrets - padding with zeros is cryptographically weak
		return nil, ErrPasetoKeyTooShort
	}

	return &AuthService{
		repo:        repo,
		revokedRepo: revokedRepo,
		pasetoKey:   key,
		tokenTTL:    tokenTTL,
		logger:      logger,
		v2:          paseto.NewV2(),
		lockout:     NewAccountLockout(lockoutThreshold, lockoutDuration),
	}, nil
}

// ErrInvalidCredentials is returned for all authentication failures to prevent user enumeration
var ErrInvalidCredentials = fmt.Errorf("invalid credentials")

func (s *AuthService) Login(username, password string) (string, *data.User, error) {
	// Check if account is locked out
	// SECURITY: Return generic error to prevent timing attacks and lockout enumeration
	if s.lockout.IsLocked(username) {
		remaining := s.lockout.GetRemainingLockoutTime(username)
		s.logger.Warn("Login attempt on locked account",
			zap.String("username", username),
			zap.Duration("remaining_lockout", remaining),
		)
		return "", nil, ErrInvalidCredentials
	}

	user, err := s.repo.GetByUsername(username)
	if err != nil {
		// Use constant-time-ish behavior: still record failure and log generically
		s.lockout.RecordFailure(username)
		s.logger.Debug("Login failed", zap.String("username", username))
		return "", nil, ErrInvalidCredentials
	}

	if err := s.checkPassword(user.Password, password); err != nil {
		locked := s.lockout.RecordFailure(username)
		if locked {
			s.logger.Warn("Account locked due to failed attempts", zap.String("username", username))
		} else {
			s.logger.Debug("Login failed", zap.String("username", username))
		}
		return "", nil, ErrInvalidCredentials
	}

	// Clear failed attempts on successful login
	s.lockout.RecordSuccess(username)

	token, err := s.generateToken(user)
	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		return "", nil, fmt.Errorf("failed to generate token")
	}

	if err := s.repo.UpdateLastLogin(user.ID); err != nil {
		s.logger.Warn("Failed to update last login time", zap.Uint("user_id", user.ID), zap.Error(err))
	}

	s.logger.Info("User logged in", zap.String("username", username), zap.Uint("user_id", user.ID))
	return token, user, nil
}

// StartLockoutCleanup starts a background goroutine to clean up old lockout entries
func (s *AuthService) StartLockoutCleanup(interval time.Duration, done <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.lockout.Cleanup()
			case <-done:
				return
			}
		}
	}()
}

func (s *AuthService) ValidateToken(token string) (*UserPayload, error) {
	tokenHash := s.hashToken(token)

	isRevoked, err := s.revokedRepo.IsRevoked(tokenHash)
	if err != nil {
		s.logger.Error("Failed to check token revocation", zap.Error(err))
		return nil, fmt.Errorf("failed to validate token")
	}
	if isRevoked {
		s.logger.Warn("Token is revoked", zap.String("token_hash", tokenHash))
		return nil, fmt.Errorf("token is revoked")
	}

	var payload UserPayload

	err = s.v2.Decrypt(token, s.pasetoKey, &payload, nil)
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

func (s *AuthService) RevokeToken(token string, reason string) error {
	tokenHash := s.hashToken(token)

	var payload UserPayload
	if err := s.v2.Decrypt(token, s.pasetoKey, &payload, nil); err == nil {
		revokedToken := &data.RevokedToken{
			TokenHash: tokenHash,
			ExpiresAt: time.Unix(payload.ExpiresAt, 0),
			Reason:    reason,
		}
		if err := s.revokedRepo.Create(revokedToken); err != nil {
			return fmt.Errorf("failed to revoke token: %w", err)
		}
		s.logger.Info("Token revoked", zap.String("token_hash", tokenHash), zap.String("reason", reason))
	}
	return nil
}

func (s *AuthService) hashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
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
