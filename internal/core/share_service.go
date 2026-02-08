package core

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ShareSceneData holds the scene data returned when resolving a share link.
type ShareSceneData struct {
	ID          uint       `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Duration    int        `json:"duration"`
	Studio      string     `json:"studio"`
	Tags        []string   `json:"tags"`
	Actors      []string   `json:"actors"`
	CreatedAt   time.Time  `json:"created_at"`
	ReleaseDate *time.Time `json:"release_date"`
}

// ResolvedShareLink holds the resolved share link with scene data.
type ResolvedShareLink struct {
	ShareLink data.ShareLink `json:"share_link"`
	Scene     ShareSceneData `json:"scene"`
}

type ShareService struct {
	shareLinkRepo data.ShareLinkRepository
	sceneRepo     data.SceneRepository
	logger        *zap.Logger
}

func NewShareService(
	shareLinkRepo data.ShareLinkRepository,
	sceneRepo data.SceneRepository,
	logger *zap.Logger,
) *ShareService {
	return &ShareService{
		shareLinkRepo: shareLinkRepo,
		sceneRepo:     sceneRepo,
		logger:        logger,
	}
}

var validExpirations = map[string]time.Duration{
	"1h":    time.Hour,
	"24h":   24 * time.Hour,
	"7d":    7 * 24 * time.Hour,
	"30d":   30 * 24 * time.Hour,
	"never": 0,
}

// CreateShareLink generates a new share link for a scene.
func (s *ShareService) CreateShareLink(userID, sceneID uint, shareType, expiresIn string) (*data.ShareLink, error) {
	if !data.IsValidShareType(shareType) {
		return nil, apperrors.NewValidationErrorWithField("share_type", fmt.Sprintf("invalid share type: %s", shareType))
	}

	dur, ok := validExpirations[expiresIn]
	if !ok {
		return nil, apperrors.NewValidationErrorWithField("expires_in", fmt.Sprintf("invalid expiration: %s", expiresIn))
	}

	// Verify scene exists
	scene, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrSceneNotFound(sceneID)
		}
		return nil, apperrors.NewInternalError("failed to verify scene", err)
	}
	if scene == nil {
		return nil, apperrors.ErrSceneNotFound(sceneID)
	}

	token, err := generateToken()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to generate share token", err)
	}

	link := &data.ShareLink{
		Token:     token,
		SceneID:   sceneID,
		UserID:    userID,
		ShareType: shareType,
	}

	if dur > 0 {
		exp := time.Now().Add(dur)
		link.ExpiresAt = &exp
	}

	if err := s.shareLinkRepo.Create(link); err != nil {
		return nil, apperrors.NewInternalError("failed to create share link", err)
	}

	s.logger.Info("share link created",
		zap.Uint("user_id", userID),
		zap.Uint("scene_id", sceneID),
		zap.String("share_type", shareType),
		zap.String("expires_in", expiresIn),
	)

	return link, nil
}

// ListShareLinks returns the caller's share links for a scene.
func (s *ShareService) ListShareLinks(userID, sceneID uint) ([]data.ShareLink, error) {
	links, err := s.shareLinkRepo.ListBySceneAndUser(sceneID, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list share links", err)
	}
	return links, nil
}

// DeleteShareLink deletes a share link owned by the user.
func (s *ShareService) DeleteShareLink(linkID, userID uint) error {
	if err := s.shareLinkRepo.Delete(linkID, userID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperrors.NewNotFoundError("share_link", linkID)
		}
		return apperrors.NewInternalError("failed to delete share link", err)
	}
	return nil
}

// ResolveShareLink resolves a token to the shared scene data.
// It checks expiry and increments view count. For auth_required type,
// isAuthenticated must be true or an error is returned.
func (s *ShareService) ResolveShareLink(token string, isAuthenticated bool) (*ResolvedShareLink, error) {
	link, err := s.shareLinkRepo.GetByToken(token)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrShareLinkNotFound(token)
		}
		return nil, apperrors.NewInternalError("failed to resolve share link", err)
	}

	// Check expiry
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return nil, apperrors.ErrShareLinkExpired
	}

	// Check auth requirement
	if link.ShareType == data.ShareTypeAuthRequired && !isAuthenticated {
		return nil, apperrors.ErrShareLinkAuthRequired
	}

	// Increment view count (best-effort, don't fail on error)
	if err := s.shareLinkRepo.IncrementViewCount(link.ID); err != nil {
		s.logger.Warn("failed to increment share link view count",
			zap.Uint("link_id", link.ID),
			zap.Error(err),
		)
	}

	// Fetch scene
	scene, err := s.sceneRepo.GetByID(link.SceneID)
	if err != nil {
		return nil, apperrors.ErrSceneNotFound(link.SceneID)
	}

	sceneData := ShareSceneData{
		ID:          scene.ID,
		Title:       scene.Title,
		Description: scene.Description,
		Duration:    scene.Duration,
		Studio:      scene.Studio,
		CreatedAt:   scene.CreatedAt,
		ReleaseDate: scene.ReleaseDate,
	}

	if scene.Tags != nil {
		sceneData.Tags = []string(scene.Tags)
	} else {
		sceneData.Tags = []string{}
	}

	if scene.Actors != nil {
		sceneData.Actors = []string(scene.Actors)
	} else {
		sceneData.Actors = []string{}
	}

	return &ResolvedShareLink{
		ShareLink: *link,
		Scene:     sceneData,
	}, nil
}

// GetShareLinkByToken fetches a share link by token without incrementing views.
// Used by streaming and OG middleware.
func (s *ShareService) GetShareLinkByToken(token string) (*data.ShareLink, error) {
	link, err := s.shareLinkRepo.GetByToken(token)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrShareLinkNotFound(token)
		}
		return nil, apperrors.NewInternalError("failed to get share link", err)
	}
	return link, nil
}

// generateToken creates a URL-safe random token (22 characters, base64url encoding of 16 random bytes).
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
