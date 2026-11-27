package workers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Worker tokens: "ghw_<worker_id>_<32 random bytes, base64url>". Only the sha256 is stored.
const tokenPrefix = "ghw_"

// NewToken generates the token of a worker and returns it with its hash.
func NewToken(workerID uint) (token, hash string, err error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", fmt.Errorf("failed to generate worker token: %w", err)
	}
	token = fmt.Sprintf("%s%d_%s", tokenPrefix, workerID, base64.RawURLEncoding.EncodeToString(secret))
	return token, HashToken(token), nil
}

// HashToken returns the stored form of a token (sha256, lowercase hex).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ParseToken extracts the worker ID of a token.
func ParseToken(token string) (uint, bool) {
	rest, ok := strings.CutPrefix(token, tokenPrefix)
	if !ok {
		return 0, false
	}
	idPart, secret, ok := strings.Cut(rest, "_")
	if !ok || secret == "" {
		return 0, false
	}
	id, err := strconv.ParseUint(idPart, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

// TokenMatches compares a token with a stored hash in constant time.
func TokenMatches(token string, hash *string) bool {
	if hash == nil || *hash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(HashToken(token)), []byte(*hash)) == 1
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// NewMediaSecret generates an HMAC secret for the signed media URLs (hex of 32 bytes).
func NewMediaSecret() (string, error) {
	s, err := randomHex(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate the media URL secret: %w", err)
	}
	return s, nil
}
