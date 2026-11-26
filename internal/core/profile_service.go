package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// ProfileService serves the profile pages (docs/PROFILES.md): the header (display name, bio,
// avatar, banner), the blocks a viewer may see, and the owner's edits. What a viewer sees is
// decided by the owner's privacy settings (PrivacyService, no administrator bypass) and, for
// the blocks showing library content, by the viewer's own permissions.
type ProfileService struct {
	repo    data.ProfileRepository
	social  *SocialService
	privacy *PrivacyService
	deps    ProfileBlockDeps
	cfg     config.ProfilesConfig
	logger  *zap.Logger

	mu             sync.Mutex
	uploadLimiters map[uint]*rate.Limiter
}

func NewProfileService(repo data.ProfileRepository, socialRepo data.SocialRepository, markerRepo data.MarkerRepository, social *SocialService, privacy *PrivacyService, homepage *HomepageService, cfg config.ProfilesConfig, logger *zap.Logger) *ProfileService {
	if cfg.MaxShowcaseItems < 1 {
		cfg.MaxShowcaseItems = 4
	}
	if cfg.MaxShowcaseItems > 12 {
		cfg.MaxShowcaseItems = 12
	}
	if cfg.BioMaxLength <= 0 {
		cfg.BioMaxLength = 1000
	}
	if cfg.BioMaxLength > 5000 {
		cfg.BioMaxLength = 5000
	}
	if cfg.DisplayNameMaxLength <= 0 || cfg.DisplayNameMaxLength > 64 {
		cfg.DisplayNameMaxLength = 64
	}
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = 8 << 20
	}
	if cfg.MaxImagePixels <= 0 {
		cfg.MaxImagePixels = 25_000_000
	}
	if cfg.ImageDir == "" {
		cfg.ImageDir = "./data/metadata/profiles"
	}
	return &ProfileService{
		repo:    repo,
		social:  social,
		privacy: privacy,
		deps: ProfileBlockDeps{
			Repo:     repo,
			Social:   socialRepo,
			Markers:  markerRepo,
			Homepage: homepage,
		},
		cfg:            cfg,
		logger:         logger.With(zap.String("service", "profile")),
		uploadLimiters: make(map[uint]*rate.Limiter),
	}
}

// Config returns the effective profile limits.
func (s *ProfileService) Config() config.ProfilesConfig { return s.cfg }

// PermissionHolder tells whether a permission is granted (*UserAccess).
type PermissionHolder interface {
	Has(permission string) bool
}

// ProfileViewer is who looks at a profile: their id and their permissions.
type ProfileViewer struct {
	UserID uint
	Access PermissionHolder
}

// Has reports whether the viewer has a permission.
func (v ProfileViewer) Has(permission string) bool {
	return v.Access != nil && v.Access.Has(permission)
}

// ProfileBanner is the banner of a profile as a viewer gets it.
type ProfileBanner struct {
	Type    string
	SceneID uint
	Version int64
}

// ProfileBlockView is a block of a profile as a viewer gets it: Locked (a "private"
// placeholder, no data), Data, or Failed (the block could not be loaded).
type ProfileBlockView struct {
	Spec   ProfileBlockSpec
	Config ProfileBlockValues
	Locked bool
	Failed bool
	Data   *ProfileBlockData
}

// ProfileView is a profile page as a viewer sees it. Bio, Banner and Blocks are only set when
// the viewer may see the profile (Social.ProfileVisible).
type ProfileView struct {
	Social *SocialProfile
	Bio    string
	Banner *ProfileBanner
	Blocks []ProfileBlockView
}

// ProfileEditState is what the owner edits: the profile, the full layout, the showcases and
// the privacy settings of the blocks.
type ProfileEditState struct {
	User            data.SocialUser
	Profile         *data.UserProfile
	Layout          data.ProfileLayout
	FavoriteScenes  []data.Scene
	FavoriteMarkers []data.WidgetMarker
	Privacy         *PrivacySettings
}

// UpdateProfileInput changes the display name and/or the bio (nil = unchanged).
type UpdateProfileInput struct {
	DisplayName *string
	Bio         *string
}

func newProfileValidationError(field, msg string) error {
	return apperrors.NewValidationErrorWithField(field, msg)
}

// Get returns the profile of username as the viewer sees it. A hidden block is omitted, or
// sent as a locked placeholder when its owner chose so; a block whose permission the viewer
// lacks is always omitted.
func (s *ProfileService) Get(ctx context.Context, viewer ProfileViewer, username string) (*ProfileView, error) {
	sp, err := s.social.Profile(ctx, viewer.UserID, username)
	if err != nil {
		return nil, err
	}
	view := &ProfileView{Social: sp, Blocks: []ProfileBlockView{}}
	if !sp.ProfileVisible {
		return view, nil
	}
	ownerID := sp.User.ID
	profile, err := s.repo.GetProfile(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the profile: %w", err)
	}
	view.Bio = profile.Bio
	if view.Banner, err = s.bannerFor(ctx, viewer, profile); err != nil {
		return nil, err
	}

	layout := NormalizeProfileLayout(profile.Layout)
	var items []string
	seen := map[string]bool{}
	for _, b := range layout {
		if spec, ok := profileBlocks[b.Type]; ok && b.Visible && !seen[spec.PrivacyItem] {
			seen[spec.PrivacyItem] = true
			items = append(items, spec.PrivacyItem)
		}
	}
	visible, err := s.privacy.Visibility(ctx, viewer.UserID, ownerID, items)
	if err != nil {
		return nil, err
	}
	for _, b := range layout {
		spec, ok := profileBlocks[b.Type]
		if !ok || !b.Visible {
			continue
		}
		if spec.ViewerPermission != "" && !viewer.Has(spec.ViewerPermission) {
			continue
		}
		values := spec.resolve(b.Config)
		if !visible[spec.PrivacyItem] {
			if b.LockedPlaceholder {
				view.Blocks = append(view.Blocks, ProfileBlockView{Spec: spec, Config: values, Locked: true})
			}
			continue
		}
		blockData, err := spec.Fetch(ctx, &s.deps, ProfileBlockRequest{OwnerID: ownerID, Viewer: viewer, Config: values})
		if err != nil {
			s.logger.Warn("profile block failed", zap.String("block", spec.Type), zap.Uint("owner_id", ownerID), zap.Error(err))
			view.Blocks = append(view.Blocks, ProfileBlockView{Spec: spec, Config: values, Failed: true})
			continue
		}
		view.Blocks = append(view.Blocks, ProfileBlockView{Spec: spec, Config: values, Data: blockData})
	}
	return view, nil
}

// bannerFor returns the banner a viewer gets: a scene banner needs scenes:view and a scene
// still in the library.
func (s *ProfileService) bannerFor(ctx context.Context, viewer ProfileViewer, p *data.UserProfile) (*ProfileBanner, error) {
	switch p.BannerType {
	case data.ProfileBannerUpload:
		if p.BannerVersion > 0 {
			return &ProfileBanner{Type: data.ProfileBannerUpload, Version: p.BannerVersion}, nil
		}
	case data.ProfileBannerScene:
		if p.BannerSceneID == nil || !viewer.Has(PermScenesView) {
			return nil, nil
		}
		live, err := s.repo.LiveSceneIDs(ctx, []uint{*p.BannerSceneID})
		if err != nil {
			return nil, fmt.Errorf("failed to check the banner scene: %w", err)
		}
		if live[*p.BannerSceneID] {
			return &ProfileBanner{Type: data.ProfileBannerScene, SceneID: *p.BannerSceneID}, nil
		}
	}
	return nil, nil
}

// EditState returns what the owner edits.
func (s *ProfileService) EditState(ctx context.Context, userID uint) (*ProfileEditState, error) {
	user, err := s.social.userByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the profile: %w", err)
	}
	scenes, err := s.favoriteScenes(ctx, userID)
	if err != nil {
		return nil, err
	}
	markers, err := s.favoriteMarkers(ctx, userID)
	if err != nil {
		return nil, err
	}
	privacy, err := s.privacy.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &ProfileEditState{
		User:            *user,
		Profile:         profile,
		Layout:          NormalizeProfileLayout(profile.Layout),
		FavoriteScenes:  scenes,
		FavoriteMarkers: markers,
		Privacy:         privacy,
	}, nil
}

func (s *ProfileService) favoriteScenes(ctx context.Context, userID uint) ([]data.Scene, error) {
	ids, err := s.repo.FavoriteSceneIDs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the favorite videos: %w", err)
	}
	scenes, err := s.repo.ScenesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to load the favorite videos: %w", err)
	}
	return scenes, nil
}

func (s *ProfileService) favoriteMarkers(ctx context.Context, userID uint) ([]data.WidgetMarker, error) {
	ids, err := s.repo.FavoriteMarkerIDs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the favorite markers: %w", err)
	}
	markers, err := s.repo.VisibleMarkers(ctx, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to load the favorite markers: %w", err)
	}
	return withMarkerTags(&s.deps, markers), nil
}

// cleanText removes the control characters of a text (keeping line breaks and tabs when
// multiline) and trims it.
func cleanText(s string, multiline bool) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			if multiline {
				return r
			}
			return ' '
		}
		if unicode.IsControl(r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, s)
	if multiline {
		// At most two consecutive empty lines
		for strings.Contains(s, "\n\n\n\n") {
			s = strings.ReplaceAll(s, "\n\n\n\n", "\n\n\n")
		}
	}
	return strings.TrimSpace(s)
}

// UpdateProfile changes the display name and/or the bio (plain text, the links are rendered
// by the frontend).
func (s *ProfileService) UpdateProfile(ctx context.Context, userID uint, in UpdateProfileInput) (*data.UserProfile, error) {
	current, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the profile: %w", err)
	}
	name, bio := current.DisplayName, current.Bio
	if in.DisplayName != nil {
		name = cleanText(*in.DisplayName, false)
		if n := utf8.RuneCountInString(name); n > s.cfg.DisplayNameMaxLength {
			return nil, newProfileValidationError("display_name", fmt.Sprintf("the display name is %d characters long, at most %d", n, s.cfg.DisplayNameMaxLength))
		}
	}
	if in.Bio != nil {
		bio = cleanText(*in.Bio, true)
		if n := utf8.RuneCountInString(bio); n > s.cfg.BioMaxLength {
			return nil, newProfileValidationError("bio", fmt.Sprintf("the bio is %d characters long, at most %d", n, s.cfg.BioMaxLength))
		}
	}
	if err := s.repo.SaveProfile(ctx, userID, name, bio); err != nil {
		return nil, fmt.Errorf("failed to save the profile: %w", err)
	}
	return s.repo.GetProfile(ctx, userID)
}

// SetFavoriteScenes replaces the favorite videos showcase (ids in order): at most
// max_showcase_items distinct scenes still in the library.
func (s *ProfileService) SetFavoriteScenes(ctx context.Context, userID uint, sceneIDs []uint) ([]data.Scene, error) {
	if err := s.checkShowcaseIDs("scene_ids", sceneIDs); err != nil {
		return nil, err
	}
	live, err := s.repo.LiveSceneIDs(ctx, sceneIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to check the scenes: %w", err)
	}
	for _, id := range sceneIDs {
		if !live[id] {
			return nil, newProfileValidationError("scene_ids", fmt.Sprintf("scene %d does not exist", id))
		}
	}
	if err := s.repo.SetFavoriteScenes(ctx, userID, sceneIDs); err != nil {
		return nil, fmt.Errorf("failed to save the favorite videos: %w", err)
	}
	return s.favoriteScenes(ctx, userID)
}

// SetFavoriteMarkers replaces the favorite markers showcase: markers the user sees (their own
// or shared with them) on scenes still in the library.
func (s *ProfileService) SetFavoriteMarkers(ctx context.Context, userID uint, markerIDs []uint) ([]data.WidgetMarker, error) {
	if err := s.checkShowcaseIDs("marker_ids", markerIDs); err != nil {
		return nil, err
	}
	visible, err := s.repo.VisibleMarkers(ctx, userID, markerIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to check the markers: %w", err)
	}
	found := make(map[uint]bool, len(visible))
	for _, m := range visible {
		found[m.ID] = true
	}
	for _, id := range markerIDs {
		if !found[id] {
			return nil, newProfileValidationError("marker_ids", fmt.Sprintf("marker %d does not exist", id))
		}
	}
	if err := s.repo.SetFavoriteMarkers(ctx, userID, markerIDs); err != nil {
		return nil, fmt.Errorf("failed to save the favorite markers: %w", err)
	}
	return s.favoriteMarkers(ctx, userID)
}

func (s *ProfileService) checkShowcaseIDs(field string, ids []uint) error {
	if len(ids) > s.cfg.MaxShowcaseItems {
		return newProfileValidationError(field, fmt.Sprintf("at most %d items", s.cfg.MaxShowcaseItems))
	}
	seen := make(map[uint]bool, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			return newProfileValidationError(field, "the items must be distinct ids")
		}
		seen[id] = true
	}
	return nil
}

// SetLayout validates and stores the block layout; nil resets it to the default layout.
func (s *ProfileService) SetLayout(ctx context.Context, userID uint, layout data.ProfileLayout) (data.ProfileLayout, error) {
	if layout == nil {
		if err := s.repo.SaveLayout(ctx, userID, nil); err != nil {
			return nil, fmt.Errorf("failed to save the layout: %w", err)
		}
		return DefaultProfileLayout(), nil
	}
	normalized, err := ValidateProfileLayout(layout)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveLayout(ctx, userID, normalized); err != nil {
		return nil, fmt.Errorf("failed to save the layout: %w", err)
	}
	return normalized, nil
}

// MarkerCandidates lists the markers the user may put in their favorite markers showcase.
func (s *ProfileService) MarkerCandidates(ctx context.Context, userID uint, query string, page, limit int) ([]data.WidgetMarker, int64, error) {
	if len(query) > 100 {
		query = query[:100]
	}
	markers, total, err := s.repo.MarkerCandidates(ctx, userID, query, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list the markers: %w", err)
	}
	return withMarkerTags(&s.deps, markers), total, nil
}

// --- Banner ---

// SetBannerScene uses the thumbnail of a scene as the banner (an uploaded banner is deleted).
func (s *ProfileService) SetBannerScene(ctx context.Context, userID, sceneID uint) error {
	live, err := s.repo.LiveSceneIDs(ctx, []uint{sceneID})
	if err != nil {
		return fmt.Errorf("failed to check the scene: %w", err)
	}
	if !live[sceneID] {
		return newProfileValidationError("scene_id", fmt.Sprintf("scene %d does not exist", sceneID))
	}
	if err := s.repo.SetBanner(ctx, userID, data.ProfileBannerScene, &sceneID, 0); err != nil {
		return fmt.Errorf("failed to save the banner: %w", err)
	}
	s.removeImages("banners", userID, ProfileBannerSizes)
	return nil
}

// ClearBanner removes the banner (and the uploaded banner files).
func (s *ProfileService) ClearBanner(ctx context.Context, userID uint) error {
	if err := s.repo.SetBanner(ctx, userID, data.ProfileBannerNone, nil, 0); err != nil {
		return fmt.Errorf("failed to save the banner: %w", err)
	}
	s.removeImages("banners", userID, ProfileBannerSizes)
	return nil
}

// UploadBanner stores an uploaded banner (cropped to 3:1) and returns its version.
func (s *ProfileService) UploadBanner(ctx context.Context, userID uint, r io.Reader, crop ProfileImageCrop) (int64, error) {
	version, err := s.storeImage(userID, "banners", r, crop, ProfileBannerSizes)
	if err != nil {
		return 0, err
	}
	if err := s.repo.SetBanner(ctx, userID, data.ProfileBannerUpload, nil, version); err != nil {
		return 0, fmt.Errorf("failed to save the banner: %w", err)
	}
	return version, nil
}

// --- Avatar ---

// UploadAvatar stores an uploaded avatar (cropped square) and returns its version.
func (s *ProfileService) UploadAvatar(ctx context.Context, userID uint, r io.Reader, crop ProfileImageCrop) (int64, error) {
	version, err := s.storeImage(userID, "avatars", r, crop, ProfileAvatarSizes)
	if err != nil {
		return 0, err
	}
	if err := s.repo.SetAvatarVersion(ctx, userID, version); err != nil {
		return 0, fmt.Errorf("failed to save the avatar: %w", err)
	}
	return version, nil
}

// DeleteAvatar goes back to the initials avatar.
func (s *ProfileService) DeleteAvatar(ctx context.Context, userID uint) error {
	if err := s.repo.SetAvatarVersion(ctx, userID, 0); err != nil {
		return fmt.Errorf("failed to save the avatar: %w", err)
	}
	s.removeImages("avatars", userID, ProfileAvatarSizes)
	return nil
}

// AvatarPath returns the file of an avatar size (it may not exist).
func (s *ProfileService) AvatarPath(userID uint, size string) (string, error) {
	return s.imagePath("avatars", userID, size, ProfileAvatarSizes)
}

// BannerPath returns the file of an uploaded banner size, if the viewer may see the profile.
func (s *ProfileService) BannerPath(ctx context.Context, viewerID, ownerID uint, size string) (string, error) {
	ok, err := s.privacy.CanView(ctx, viewerID, ownerID, PrivacyProfile)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", apperrors.ErrPrivacyRestricted()
	}
	return s.imagePath("banners", ownerID, size, ProfileBannerSizes)
}

func (s *ProfileService) imagePath(kind string, userID uint, size string, sizes []ProfileImageSize) (string, error) {
	for _, sz := range sizes {
		if sz.Name == size {
			return filepath.Join(s.cfg.ImageDir, kind, fmt.Sprintf("%d_%s.jpg", userID, size)), nil
		}
	}
	return "", newProfileValidationError("size", fmt.Sprintf("unknown image size %q", size))
}

// allowUpload applies the per-user upload rate limit.
func (s *ProfileService) allowUpload(userID uint) bool {
	perHour := s.cfg.ImageUploadsPerHour
	if perHour <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.uploadLimiters[userID]
	if !ok {
		burst := perHour / 2
		if burst < 1 {
			burst = 1
		}
		l = rate.NewLimiter(rate.Every(time.Hour/time.Duration(perHour)), burst)
		s.uploadLimiters[userID] = l
	}
	return l.Allow()
}

// storeImage processes an upload and writes its sizes (each file replaced atomically).
func (s *ProfileService) storeImage(userID uint, kind string, r io.Reader, crop ProfileImageCrop, sizes []ProfileImageSize) (int64, error) {
	if !s.allowUpload(userID) {
		return 0, apperrors.ErrSocialRateLimited()
	}
	files, err := ProcessProfileImage(r, ProfileImageLimits{MaxBytes: s.cfg.MaxUploadBytes, MaxPixels: s.cfg.MaxImagePixels}, crop, sizes)
	if err != nil {
		return 0, err
	}
	dir := filepath.Join(s.cfg.ImageDir, kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("failed to create the profile image directory: %w", err)
	}
	for _, size := range sizes {
		path := filepath.Join(dir, fmt.Sprintf("%d_%s.jpg", userID, size.Name))
		if err := writeFileAtomic(path, files[size.Name]); err != nil {
			return 0, err
		}
	}
	return time.Now().UnixMilli(), nil
}

// removeImages deletes the profile image files of a user (GoonHub data, not media). Best
// effort: a leftover file is never served once the version is reset.
func (s *ProfileService) removeImages(kind string, userID uint, sizes []ProfileImageSize) {
	for _, size := range sizes {
		path := filepath.Join(s.cfg.ImageDir, kind, fmt.Sprintf("%d_%s.jpg", userID, size.Name))
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Warn("failed to remove a profile image", zap.String("path", path), zap.Error(err))
		}
	}
}

// writeFileAtomic writes a file through a temporary file and a rename.
func writeFileAtomic(path string, content []byte) error {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return fmt.Errorf("failed to name a temporary file: %w", err)
	}
	tmp := path + ".tmp-" + hex.EncodeToString(suffix[:])
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return fmt.Errorf("failed to write the profile image: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to write the profile image: %w", err)
	}
	return nil
}
