package core

import (
	"bytes"
	"context"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type profileTestEnv struct {
	svc     *ProfileService
	repo    *mocks.MockProfileRepository
	social  *mocks.MockSocialRepository
	privacy *mocks.MockPrivacyRepository
}

func newProfileTestEnv(t *testing.T, cfg config.ProfilesConfig) *profileTestEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockProfileRepository(ctrl)
	socialRepo := mocks.NewMockSocialRepository(ctrl)
	privacyRepo := mocks.NewMockPrivacyRepository(ctrl)
	privacy := NewPrivacyService(privacyRepo, socialRepo, zap.NewNop())
	social := NewSocialService(socialRepo, privacy, NewEventBus(zap.NewNop()), config.SocialConfig{}, zap.NewNop())
	svc := NewProfileService(repo, socialRepo, nil, social, privacy, nil, cfg, zap.NewNop())
	return &profileTestEnv{svc: svc, repo: repo, social: socialRepo, privacy: privacyRepo}
}

// owner bob (5): favorites public, stats private, activity friends (default), friends list
// public (default). The viewers are not related to bob.
func (e *profileTestEnv) expectBob(settings []data.UserPrivacySetting) {
	e.social.EXPECT().GetUserByUsername(gomock.Any(), "bob").Return(&data.SocialUser{ID: 5, Username: "bob", DisplayName: "Bob", AvatarVersion: 7}, nil).AnyTimes()
	e.social.EXPECT().Relations(gomock.Any(), gomock.Any(), []uint{5}).Return([]data.UserRelation{{UserID: 5}}, nil).AnyTimes()
	e.social.EXPECT().Counts(gomock.Any(), uint(5)).Return(data.SocialCounts{Friends: 1}, nil).AnyTimes()
	e.privacy.EXPECT().ListSettings(gomock.Any(), uint(5)).Return(settings, nil).AnyTimes()
}

var bobSettings = []data.UserPrivacySetting{
	{UserID: 5, Item: PrivacyFavorites, Audience: "public"},
	{UserID: 5, Item: PrivacyStats, Audience: "private"},
}

var bobLayout = data.ProfileLayout{
	{Type: "favorite_scenes", Visible: true},
	{Type: "stats", Visible: true, LockedPlaceholder: true},
	{Type: "recent_watches", Visible: true},
	{Type: "liked_scenes", Visible: false},
	{Type: "friends", Visible: true, Config: map[string]any{"count": 4.0}},
}

func blockTypes(v *ProfileView) []string {
	out := make([]string, len(v.Blocks))
	for i, b := range v.Blocks {
		out[i] = b.Spec.Type
		if b.Locked {
			out[i] += ":locked"
		}
	}
	return out
}

func TestProfileGetChecksPrivacyPerBlock(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob(bobSettings)
	e.repo.EXPECT().GetProfile(gomock.Any(), uint(5)).Return(&data.UserProfile{UserID: 5, Bio: "hi", BannerType: data.ProfileBannerUpload, BannerVersion: 3, Layout: bobLayout}, nil)
	e.repo.EXPECT().FavoriteSceneIDs(gomock.Any(), uint(5)).Return([]uint{10}, nil)
	e.repo.EXPECT().ScenesByIDs(gomock.Any(), []uint{10}).Return([]data.Scene{{ID: 10, Title: "fav"}}, nil)
	e.social.EXPECT().ListFriends(gomock.Any(), uint(5), 1, 4).Return([]data.SocialUser{{ID: 6, Username: "al", AvatarVersion: 2}}, int64(1), nil)
	// recent_watches (activity: friends) and stats (private) are never fetched

	viewer := ProfileViewer{UserID: 1, Access: NewUserAccess(1, "user", false, PermScenesView)}
	view, err := e.svc.Get(context.Background(), viewer, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(blockTypes(view), ","); got != "favorite_scenes,stats:locked,friends" {
		t.Fatalf("blocks = %s", got)
	}
	if view.Bio != "hi" || view.Banner == nil || view.Banner.Version != 3 {
		t.Fatalf("header: %q %+v", view.Bio, view.Banner)
	}
	if view.Blocks[1].Data != nil {
		t.Fatal("a locked block carries data")
	}
	friends := view.Blocks[2].Data.Widget.(ProfileFriendsPayload)
	if len(friends.Items) != 1 || friends.Items[0].AvatarVersion != 2 {
		t.Fatalf("friends: %+v", friends)
	}
}

func TestProfileGetNeedsViewerPermission(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob(bobSettings)
	e.repo.EXPECT().GetProfile(gomock.Any(), uint(5)).Return(&data.UserProfile{UserID: 5, BannerType: data.ProfileBannerScene, BannerSceneID: ptrUint(10), Layout: bobLayout}, nil)
	e.social.EXPECT().ListFriends(gomock.Any(), uint(5), 1, 4).Return(nil, int64(0), nil)
	// No scenes:view: no scene block (not even the locked placeholder of stats), no scene banner
	view, err := e.svc.Get(context.Background(), ProfileViewer{UserID: 1, Access: NewUserAccess(1, "guest", false)}, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(blockTypes(view), ","); got != "friends" || view.Banner != nil {
		t.Fatalf("blocks = %s, banner = %+v", got, view.Banner)
	}
	if !view.Blocks[0].Data.Empty {
		t.Fatal("an empty friends block is not marked empty")
	}
}

func TestProfileGetPrivateProfile(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob([]data.UserPrivacySetting{{UserID: 5, Item: PrivacyProfile, Audience: "private"}})
	// No GetProfile: nothing of the profile is read. A superuser is a viewer like any other.
	view, err := e.svc.Get(context.Background(), ProfileViewer{UserID: 1, Access: NewUserAccess(1, "admin", true)}, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if view.Social.ProfileVisible || len(view.Blocks) != 0 || view.Bio != "" {
		t.Fatalf("private profile leaked: %+v", view)
	}
	if view.Social.User.DisplayName != "Bob" || view.Social.User.AvatarVersion != 7 {
		t.Fatalf("identity: %+v", view.Social.User)
	}
}

func TestProfileGetOwnerSeesEverything(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.social.EXPECT().GetUserByUsername(gomock.Any(), "bob").Return(&data.SocialUser{ID: 5, Username: "bob"}, nil)
	e.social.EXPECT().Counts(gomock.Any(), uint(5)).Return(data.SocialCounts{}, nil)
	e.repo.EXPECT().GetProfile(gomock.Any(), uint(5)).Return(&data.UserProfile{UserID: 5, Layout: bobLayout}, nil)
	e.repo.EXPECT().FavoriteSceneIDs(gomock.Any(), uint(5)).Return(nil, nil)
	e.repo.EXPECT().ScenesByIDs(gomock.Any(), gomock.Any()).Return([]data.Scene{}, nil).Times(2)
	e.repo.EXPECT().Stats(gomock.Any(), uint(5), gomock.Any()).Return(&data.ProfileStats{Views: 3}, nil)
	e.repo.EXPECT().TopTags(gomock.Any(), uint(5), gomock.Any(), profileStatsTopLimit).Return([]data.ProfileTopEntry{}, nil)
	e.repo.EXPECT().TopActors(gomock.Any(), uint(5), gomock.Any(), profileStatsTopLimit).Return([]data.ProfileTopEntry{}, nil)
	e.repo.EXPECT().TopStudios(gomock.Any(), uint(5), gomock.Any(), profileStatsTopLimit).Return([]data.ProfileTopEntry{}, nil)
	e.repo.EXPECT().RecentWatches(gomock.Any(), uint(5), 8).Return(nil, nil)
	e.social.EXPECT().ListFriends(gomock.Any(), uint(5), 1, 4).Return(nil, int64(0), nil)
	view, err := e.svc.Get(context.Background(), ProfileViewer{UserID: 5, Access: NewUserAccess(5, "user", false, PermScenesView)}, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(blockTypes(view), ","); got != "favorite_scenes,stats,recent_watches,friends" {
		t.Fatalf("blocks = %s", got)
	}
	if view.Blocks[1].Data.Empty {
		t.Fatal("stats with views marked empty")
	}
}

func ptrUint(v uint) *uint { return &v }

func TestUpdateProfileValidation(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{BioMaxLength: 10, DisplayNameMaxLength: 5})
	ctx := context.Background()
	e.repo.EXPECT().GetProfile(gomock.Any(), uint(1)).Return(data.DefaultUserProfile(1), nil).AnyTimes()
	long := strings.Repeat("é", 11)
	if _, err := e.svc.UpdateProfile(ctx, 1, UpdateProfileInput{Bio: &long}); !apperrors.IsValidation(err) {
		t.Fatalf("long bio: %v", err)
	}
	name := "abcdef"
	if _, err := e.svc.UpdateProfile(ctx, 1, UpdateProfileInput{DisplayName: &name}); !apperrors.IsValidation(err) {
		t.Fatalf("long name: %v", err)
	}
	dirty, bio := " A\x00B\tC ", "x\r\n\n\n\n\ny\x07"
	e.repo.EXPECT().SaveProfile(gomock.Any(), uint(1), "AB C", "x\n\n\ny").Return(nil)
	if _, err := e.svc.UpdateProfile(ctx, 1, UpdateProfileInput{DisplayName: &dirty, Bio: &bio}); err != nil {
		t.Fatal(err)
	}
}

func TestSetFavoriteScenesValidation(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{MaxShowcaseItems: 2})
	ctx := context.Background()
	for name, ids := range map[string][]uint{"too many": {1, 2, 3}, "duplicate": {1, 1}, "zero": {0}} {
		if _, err := e.svc.SetFavoriteScenes(ctx, 1, ids); !apperrors.IsValidation(err) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	e.repo.EXPECT().LiveSceneIDs(gomock.Any(), []uint{4, 9}).Return(map[uint]bool{4: true}, nil)
	if _, err := e.svc.SetFavoriteScenes(ctx, 1, []uint{4, 9}); !apperrors.IsValidation(err) {
		t.Fatalf("trashed scene: %v", err)
	}
	e.repo.EXPECT().LiveSceneIDs(gomock.Any(), []uint{9, 4}).Return(map[uint]bool{4: true, 9: true}, nil)
	e.repo.EXPECT().SetFavoriteScenes(gomock.Any(), uint(1), []uint{9, 4}).Return(nil)
	e.repo.EXPECT().FavoriteSceneIDs(gomock.Any(), uint(1)).Return([]uint{9, 4}, nil)
	e.repo.EXPECT().ScenesByIDs(gomock.Any(), []uint{9, 4}).Return([]data.Scene{{ID: 9}, {ID: 4}}, nil)
	if scenes, err := e.svc.SetFavoriteScenes(ctx, 1, []uint{9, 4}); err != nil || len(scenes) != 2 {
		t.Fatalf("set: %v %v", scenes, err)
	}
}

func TestSetFavoriteMarkersOnlyVisibleOnes(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.repo.EXPECT().VisibleMarkers(gomock.Any(), uint(1), []uint{3, 8}).Return([]data.WidgetMarker{{ID: 3}}, nil)
	if _, err := e.svc.SetFavoriteMarkers(context.Background(), 1, []uint{3, 8}); !apperrors.IsValidation(err) {
		t.Fatalf("hidden marker accepted: %v", err)
	}
}

func TestAvatarUploadAndDelete(t *testing.T) {
	dir := t.TempDir()
	e := newProfileTestEnv(t, config.ProfilesConfig{ImageDir: dir, ImageUploadsPerHour: 2})
	ctx := context.Background()
	img := encodePNG(t, solidImage(300, 200, color.White))
	e.repo.EXPECT().SetAvatarVersion(gomock.Any(), uint(3), gomock.Any()).Return(nil)
	version, err := e.svc.UploadAvatar(ctx, 3, bytes.NewReader(img), ProfileImageCrop{})
	if err != nil || version <= 0 {
		t.Fatalf("upload: %d %v", version, err)
	}
	for _, size := range ProfileAvatarSizes {
		path, err := e.svc.AvatarPath(3, size.Name)
		if err != nil || filepath.Dir(path) != filepath.Join(dir, "avatars") {
			t.Fatalf("path %s: %s %v", size.Name, path, err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s not written: %v", size.Name, err)
		}
	}
	if _, err := e.svc.AvatarPath(3, "../x"); !apperrors.IsValidation(err) {
		t.Fatalf("bad size accepted: %v", err)
	}
	// The second upload in a row is over the rate limit (2 per hour: burst of 1)
	if _, err := e.svc.UploadAvatar(ctx, 3, bytes.NewReader(img), ProfileImageCrop{}); err == nil {
		t.Fatal("rate limit not applied")
	}
	// A rejected image changes nothing
	other := newProfileTestEnv(t, config.ProfilesConfig{ImageDir: dir})
	if _, err := other.svc.UploadAvatar(ctx, 4, strings.NewReader("not an image"), ProfileImageCrop{}); !apperrors.IsValidation(err) {
		t.Fatalf("non image: %v", err)
	}
	e.repo.EXPECT().SetAvatarVersion(gomock.Any(), uint(3), int64(0)).Return(nil)
	if err := e.svc.DeleteAvatar(ctx, 3); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "avatars"))
	if len(entries) != 0 {
		t.Fatalf("files left: %v", entries)
	}
}

func TestBannerPathNeedsProfileVisibility(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{ImageDir: t.TempDir()})
	e.privacy.EXPECT().AudiencesFor(gomock.Any(), []uint{5}, PrivacyProfile).Return(map[uint]string{5: "private"}, nil)
	if _, err := e.svc.BannerPath(context.Background(), 1, 5, "lg"); !apperrors.IsPrivacyRestricted(err) {
		t.Fatalf("private profile banner: %v", err)
	}
	if path, err := e.svc.BannerPath(context.Background(), 5, 5, "md"); err != nil || !strings.HasSuffix(path, "5_md.jpg") {
		t.Fatalf("own banner: %s %v", path, err)
	}
}
