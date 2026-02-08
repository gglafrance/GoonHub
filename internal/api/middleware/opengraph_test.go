package middleware

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/infrastructure/logging"
	"goonhub/internal/mocks"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupOGTest(t *testing.T) (*OGMiddleware, *mocks.MockSceneRepository, *mocks.MockActorRepository, *mocks.MockStudioRepository, *mocks.MockPlaylistRepository) {
	ctrl := gomock.NewController(t)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	actorRepo := mocks.NewMockActorRepository(ctrl)
	studioRepo := mocks.NewMockStudioRepository(ctrl)
	playlistRepo := mocks.NewMockPlaylistRepository(ctrl)
	logger := &logging.Logger{Logger: zap.NewNop()}

	mw := NewOGMiddleware(sceneRepo, actorRepo, studioRepo, playlistRepo, logger)
	return mw, sceneRepo, actorRepo, studioRepo, playlistRepo
}

func makeRequest(path string, userAgent string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	c.Request.Header.Set("User-Agent", userAgent)
	return c, w
}

const discordUA = "Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)"
const browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36"

func TestNonCrawlerPassesThrough(t *testing.T) {
	mw, _, _, _, _ := setupOGTest(t)
	c, _ := makeRequest("/watch/42", browserUA)

	handled := mw.ServeIfCrawler(c)
	if handled {
		t.Fatal("expected non-crawler request to pass through")
	}
}

func TestCrawlerNonEntityPathPassesThrough(t *testing.T) {
	mw, _, _, _, _ := setupOGTest(t)
	c, _ := makeRequest("/settings", discordUA)

	handled := mw.ServeIfCrawler(c)
	if handled {
		t.Fatal("expected non-entity path to pass through")
	}
}

func TestCrawlerSceneServesOG(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(42)).Return(&data.Scene{
		ID:          42,
		Title:       "Test Scene Title",
		Description: "A description of the scene",
	}, nil)

	c, w := makeRequest("/watch/42", discordUA)
	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler scene request to be handled")
	}

	body := w.Body.String()
	if !strings.Contains(body, `og:title`) {
		t.Fatal("expected og:title in response")
	}
	if !strings.Contains(body, "Test Scene Title") {
		t.Fatal("expected scene title in response")
	}
	if !strings.Contains(body, "A description of the scene") {
		t.Fatal("expected scene description in response")
	}
	if !strings.Contains(body, "/thumbnails/42?size=lg") {
		t.Fatal("expected thumbnail URL in response")
	}
	if !strings.Contains(body, "video.other") {
		t.Fatal("expected og:type video.other in response")
	}
}

func TestCrawlerSceneFallbackToFilename(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:               1,
		Title:            "",
		OriginalFilename: "my_video.mp4",
	}, nil)

	c, w := makeRequest("/watch/1", discordUA)
	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler request to be handled")
	}

	body := w.Body.String()
	if !strings.Contains(body, "my_video.mp4") {
		t.Fatal("expected original filename as title fallback")
	}
}

func TestCrawlerSceneNotFound(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound)

	c, _ := makeRequest("/watch/999", discordUA)
	handled := mw.ServeIfCrawler(c)
	if handled {
		t.Fatal("expected not-found scene to pass through")
	}
}

func TestCrawlerInvalidSceneID(t *testing.T) {
	mw, _, _, _, _ := setupOGTest(t)
	c, _ := makeRequest("/watch/abc", discordUA)

	handled := mw.ServeIfCrawler(c)
	if handled {
		t.Fatal("expected invalid scene ID to pass through")
	}
}

func TestCrawlerActorServesOG(t *testing.T) {
	mw, _, actorRepo, _, _ := setupOGTest(t)
	actorUUID := uuid.New()

	actorRepo.EXPECT().GetByUUID(actorUUID.String()).Return(&data.Actor{
		UUID:     actorUUID,
		Name:     "Jane Doe",
		ImageURL: "jane-doe.jpg",
	}, nil)

	c, w := makeRequest("/actors/"+actorUUID.String(), discordUA)
	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler actor request to be handled")
	}

	body := w.Body.String()
	if !strings.Contains(body, "Jane Doe") {
		t.Fatal("expected actor name in response")
	}
	if !strings.Contains(body, "/actor-images/jane-doe.jpg") {
		t.Fatal("expected actor image URL in response")
	}
	if !strings.Contains(body, "profile") {
		t.Fatal("expected og:type profile in response")
	}
}

func TestCrawlerActorNoImage(t *testing.T) {
	mw, _, actorRepo, _, _ := setupOGTest(t)
	actorUUID := uuid.New()

	actorRepo.EXPECT().GetByUUID(actorUUID.String()).Return(&data.Actor{
		UUID:     actorUUID,
		Name:     "No Image Actor",
		ImageURL: "",
	}, nil)

	c, w := makeRequest("/actors/"+actorUUID.String(), discordUA)
	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler request to be handled")
	}

	body := w.Body.String()
	if strings.Contains(body, "og:image") {
		t.Fatal("expected no og:image tag when actor has no image")
	}
}

func TestCrawlerStudioServesOG(t *testing.T) {
	mw, _, _, studioRepo, _ := setupOGTest(t)
	studioUUID := uuid.New()

	studioRepo.EXPECT().GetByUUID(studioUUID.String()).Return(&data.Studio{
		UUID:        studioUUID,
		Name:        "Big Studio",
		Description: "Studio description",
		Logo:        "big-studio.png",
	}, nil)

	c, w := makeRequest("/studios/"+studioUUID.String(), discordUA)
	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler studio request to be handled")
	}

	body := w.Body.String()
	if !strings.Contains(body, "Big Studio") {
		t.Fatal("expected studio name in response")
	}
	if !strings.Contains(body, "Studio description") {
		t.Fatal("expected studio description in response")
	}
	if !strings.Contains(body, "/studio-logos/big-studio.png") {
		t.Fatal("expected studio logo URL in response")
	}
}

func TestCrawlerPublicPlaylistServesOG(t *testing.T) {
	mw, _, _, _, playlistRepo := setupOGTest(t)
	playlistUUID := uuid.New()
	desc := "My playlist description"

	playlistRepo.EXPECT().GetByUUID(playlistUUID.String()).Return(&data.Playlist{
		UUID:        playlistUUID,
		Name:        "My Playlist",
		Description: &desc,
		Visibility:  "public",
	}, nil)

	c, w := makeRequest("/playlists/"+playlistUUID.String(), discordUA)
	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler public playlist request to be handled")
	}

	body := w.Body.String()
	if !strings.Contains(body, "My Playlist") {
		t.Fatal("expected playlist name in response")
	}
	if !strings.Contains(body, "My playlist description") {
		t.Fatal("expected playlist description in response")
	}
}

func TestCrawlerPrivatePlaylistPassesThrough(t *testing.T) {
	mw, _, _, _, playlistRepo := setupOGTest(t)
	playlistUUID := uuid.New()

	playlistRepo.EXPECT().GetByUUID(playlistUUID.String()).Return(&data.Playlist{
		UUID:       playlistUUID,
		Name:       "Private Playlist",
		Visibility: "private",
	}, nil)

	c, _ := makeRequest("/playlists/"+playlistUUID.String(), discordUA)
	handled := mw.ServeIfCrawler(c)
	if handled {
		t.Fatal("expected private playlist to pass through")
	}
}

func TestCrawlerMarkerServesOG(t *testing.T) {
	mw, _, _, _, _ := setupOGTest(t)
	c, w := makeRequest("/markers/blowjob", discordUA)

	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler marker request to be handled")
	}

	body := w.Body.String()
	if !strings.Contains(body, "Markers: blowjob") {
		t.Fatal("expected marker label in title")
	}
}

func TestHTMLEscaping(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:          1,
		Title:       `<script>alert("xss")</script>`,
		Description: `He said "hello" & 'goodbye'`,
	}, nil)

	c, w := makeRequest("/watch/1", discordUA)
	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler request to be handled")
	}

	body := w.Body.String()
	if strings.Contains(body, "<script>") {
		t.Fatal("expected HTML characters to be escaped in title")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatal("expected escaped script tag in output")
	}
	if strings.Contains(body, `"hello"`) {
		t.Fatal("expected quotes to be escaped in description")
	}
}

func TestDescriptionTruncation(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	longDesc := strings.Repeat("a", 250)
	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:          1,
		Title:       "Long Desc Scene",
		Description: longDesc,
	}, nil)

	c, w := makeRequest("/watch/1", discordUA)
	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler request to be handled")
	}

	body := w.Body.String()
	// The truncated desc should be 200 chars + "..."
	expected := strings.Repeat("a", 200) + "..."
	if !strings.Contains(body, expected) {
		t.Fatal("expected description to be truncated to 200 chars with ellipsis")
	}
}

func TestBaseURLWithXForwardedProto(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:    1,
		Title: "Proto Test",
	}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/watch/1", nil)
	c.Request.Header.Set("User-Agent", discordUA)
	c.Request.Header.Set("X-Forwarded-Proto", "http")
	c.Request.Host = "example.com"

	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler request to be handled")
	}

	body := w.Body.String()
	if !strings.Contains(body, "http://example.com") {
		t.Fatal("expected base URL with X-Forwarded-Proto http scheme")
	}
}

func TestBaseURLDefaultsToHTTPS(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:    1,
		Title: "Default Proto Test",
	}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/watch/1", nil)
	c.Request.Header.Set("User-Agent", discordUA)
	c.Request.Host = "example.com"

	handled := mw.ServeIfCrawler(c)
	if !handled {
		t.Fatal("expected crawler request to be handled")
	}

	body := w.Body.String()
	if !strings.Contains(body, "https://example.com") {
		t.Fatal("expected base URL to default to https")
	}
}

func TestVariousCrawlerUserAgents(t *testing.T) {
	crawlers := []string{
		"Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)",
		"Twitterbot/1.0",
		"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
		"LinkedInBot/1.0 (compatible; Mozilla/5.0)",
		"Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
		"WhatsApp/2.19.81 A",
		"TelegramBot (like TwitterBot)",
	}

	for _, ua := range crawlers {
		t.Run(ua, func(t *testing.T) {
			mw, sceneRepo, _, _, _ := setupOGTest(t)

			sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
				ID:    1,
				Title: "Test",
			}, nil)

			c, w := makeRequest("/watch/1", ua)
			handled := mw.ServeIfCrawler(c)
			if !handled {
				t.Fatalf("expected crawler UA %q to be detected", ua)
			}
			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", w.Code)
			}
		})
	}
}

func TestCrawlerRootPathPassesThrough(t *testing.T) {
	mw, _, _, _, _ := setupOGTest(t)
	c, _ := makeRequest("/", discordUA)

	handled := mw.ServeIfCrawler(c)
	if handled {
		t.Fatal("expected root path to pass through")
	}
}

func TestCrawlerWatchWithoutIDPassesThrough(t *testing.T) {
	mw, _, _, _, _ := setupOGTest(t)
	c, _ := makeRequest("/watch", discordUA)

	handled := mw.ServeIfCrawler(c)
	if handled {
		t.Fatal("expected /watch without ID to pass through")
	}
}

func TestResponseContentType(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:    1,
		Title: "Content Type Test",
	}, nil)

	c, w := makeRequest("/watch/1", discordUA)
	mw.ServeIfCrawler(c)

	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html content type, got %q", ct)
	}
}

func TestMetaRefreshTag(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:    1,
		Title: "Refresh Test",
	}, nil)

	c, w := makeRequest("/watch/1", discordUA)
	mw.ServeIfCrawler(c)

	body := w.Body.String()
	if !strings.Contains(body, `http-equiv="refresh"`) {
		t.Fatal("expected meta refresh tag for real user redirect")
	}
}

func TestThemeColorTag(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:    1,
		Title: "Theme Test",
	}, nil)

	c, w := makeRequest("/watch/1", discordUA)
	mw.ServeIfCrawler(c)

	body := w.Body.String()
	if !strings.Contains(body, `#050505`) {
		t.Fatal("expected theme-color #050505")
	}
}

func TestTwitterCardTag(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:    1,
		Title: "Twitter Test",
	}, nil)

	c, w := makeRequest("/watch/1", discordUA)
	mw.ServeIfCrawler(c)

	body := w.Body.String()
	if !strings.Contains(body, `twitter:card`) {
		t.Fatal("expected twitter:card tag")
	}
	if !strings.Contains(body, `summary_large_image`) {
		t.Fatal("expected summary_large_image twitter card type")
	}
}

func TestDescriptionExact200CharsNotTruncated(t *testing.T) {
	desc := strings.Repeat("b", 200)
	result := truncateDescription(desc)
	if result != desc {
		t.Fatal("expected exactly 200 char description to NOT be truncated")
	}
}

func TestDescriptionEmpty(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:    1,
		Title: "No Desc",
	}, nil)

	c, w := makeRequest("/watch/1", discordUA)
	mw.ServeIfCrawler(c)

	body := w.Body.String()
	if strings.Contains(body, "og:description") {
		t.Fatal("expected no og:description when description is empty")
	}
}

func TestSceneURLFormat(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(42)).Return(&data.Scene{
		ID:    42,
		Title: "URL Test",
	}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/watch/42", nil)
	c.Request.Header.Set("User-Agent", discordUA)
	c.Request.Host = "myhost.com"

	mw.ServeIfCrawler(c)

	body := w.Body.String()
	expected := fmt.Sprintf("https://myhost.com/watch/42")
	if !strings.Contains(body, expected) {
		t.Fatalf("expected og:url %q in body", expected)
	}
}

func TestSiteNameTag(t *testing.T) {
	mw, sceneRepo, _, _, _ := setupOGTest(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:    1,
		Title: "Site Name Test",
	}, nil)

	c, w := makeRequest("/watch/1", discordUA)
	mw.ServeIfCrawler(c)

	body := w.Body.String()
	if !strings.Contains(body, `og:site_name`) {
		t.Fatal("expected og:site_name tag")
	}
	if !strings.Contains(body, "GoonHub") {
		t.Fatal("expected GoonHub as site name")
	}
}
