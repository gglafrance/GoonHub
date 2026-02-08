package middleware

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/infrastructure/logging"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var crawlerUA = regexp.MustCompile(`(?i)Discordbot|Twitterbot|facebookexternalhit|LinkedInBot|Slackbot|WhatsApp|TelegramBot`)

const maxDescriptionLen = 200

// OGMiddleware serves minimal HTML with OpenGraph meta tags for social media crawlers.
// Normal browser requests pass through unchanged (zero overhead).
type OGMiddleware struct {
	sceneRepo    data.SceneRepository
	actorRepo    data.ActorRepository
	studioRepo   data.StudioRepository
	playlistRepo data.PlaylistRepository
	logger       *logging.Logger
}

func NewOGMiddleware(
	sceneRepo data.SceneRepository,
	actorRepo data.ActorRepository,
	studioRepo data.StudioRepository,
	playlistRepo data.PlaylistRepository,
	logger *logging.Logger,
) *OGMiddleware {
	return &OGMiddleware{
		sceneRepo:    sceneRepo,
		actorRepo:    actorRepo,
		studioRepo:   studioRepo,
		playlistRepo: playlistRepo,
		logger:       logger,
	}
}

// ServeIfCrawler checks if the request is from a social media crawler and, if so,
// serves a minimal HTML page with correct OG meta tags. Returns true if the response
// was handled, false if the caller should continue with normal processing.
func (m *OGMiddleware) ServeIfCrawler(c *gin.Context) bool {
	ua := c.Request.UserAgent()
	if !crawlerUA.MatchString(ua) {
		return false
	}

	path := c.Request.URL.Path
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) < 1 {
		return false
	}

	baseURL := deriveBaseURL(c)

	switch segments[0] {
	case "watch":
		if len(segments) < 2 {
			return false
		}
		return m.serveScene(c, segments[1], baseURL)

	case "actors":
		if len(segments) < 2 {
			return false
		}
		return m.serveActor(c, segments[1], baseURL)

	case "studios":
		if len(segments) < 2 {
			return false
		}
		return m.serveStudio(c, segments[1], baseURL)

	case "playlists":
		if len(segments) < 2 {
			return false
		}
		return m.servePlaylist(c, segments[1], baseURL)

	case "markers":
		if len(segments) < 2 {
			return false
		}
		return m.serveMarker(c, segments[1], baseURL)
	}

	return false
}

func (m *OGMiddleware) serveScene(c *gin.Context, idStr string, baseURL string) bool {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return false
	}

	scene, err := m.sceneRepo.GetByID(uint(id))
	if err != nil {
		return false
	}

	title := scene.Title
	if title == "" {
		title = scene.OriginalFilename
	}

	desc := truncateDescription(scene.Description)
	image := fmt.Sprintf("%s/thumbnails/%d?size=lg", baseURL, scene.ID)
	url := fmt.Sprintf("%s/watch/%d", baseURL, scene.ID)

	renderOGPage(c, title, desc, image, url, "video.other")
	return true
}

func (m *OGMiddleware) serveActor(c *gin.Context, uuid string, baseURL string) bool {
	actor, err := m.actorRepo.GetByUUID(uuid)
	if err != nil {
		return false
	}

	var image string
	if actor.ImageURL != "" {
		image = fmt.Sprintf("%s/actor-images/%s", baseURL, actor.ImageURL)
	}

	url := fmt.Sprintf("%s/actors/%s", baseURL, actor.UUID.String())

	renderOGPage(c, actor.Name, "", image, url, "profile")
	return true
}

func (m *OGMiddleware) serveStudio(c *gin.Context, uuid string, baseURL string) bool {
	studio, err := m.studioRepo.GetByUUID(uuid)
	if err != nil {
		return false
	}

	var image string
	if studio.Logo != "" {
		image = fmt.Sprintf("%s/studio-logos/%s", baseURL, studio.Logo)
	}

	desc := truncateDescription(studio.Description)
	url := fmt.Sprintf("%s/studios/%s", baseURL, studio.UUID.String())

	renderOGPage(c, studio.Name, desc, image, url, "website")
	return true
}

func (m *OGMiddleware) servePlaylist(c *gin.Context, uuid string, baseURL string) bool {
	playlist, err := m.playlistRepo.GetByUUID(uuid)
	if err != nil {
		return false
	}

	if playlist.Visibility != "public" {
		return false
	}

	var desc string
	if playlist.Description != nil {
		desc = truncateDescription(*playlist.Description)
	}

	url := fmt.Sprintf("%s/playlists/%s", baseURL, playlist.UUID.String())

	renderOGPage(c, playlist.Name, desc, "", url, "website")
	return true
}

func (m *OGMiddleware) serveMarker(c *gin.Context, label string, baseURL string) bool {
	title := fmt.Sprintf("Markers: %s", label)
	url := fmt.Sprintf("%s/markers/%s", baseURL, label)

	renderOGPage(c, title, "", "", url, "website")
	return true
}

func deriveBaseURL(c *gin.Context) string {
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

func truncateDescription(desc string) string {
	if len(desc) <= maxDescriptionLen {
		return desc
	}
	return desc[:maxDescriptionLen] + "..."
}

func renderOGPage(c *gin.Context, title, description, image, url, ogType string) {
	safeTitle := html.EscapeString(title)
	safeDesc := html.EscapeString(description)
	safeImage := html.EscapeString(image)
	safeURL := html.EscapeString(url)

	var imageTags string
	if safeImage != "" {
		imageTags = fmt.Sprintf(
			`<meta property="og:image" content="%s" />`+"\n"+
				`    <meta name="twitter:image" content="%s" />`,
			safeImage, safeImage,
		)
	}

	var descTags string
	if safeDesc != "" {
		descTags = fmt.Sprintf(
			`<meta property="og:description" content="%s" />`+"\n"+
				`    <meta name="twitter:description" content="%s" />`,
			safeDesc, safeDesc,
		)
	}

	body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8" />
    <meta property="og:title" content="%s" />
    <meta property="og:url" content="%s" />
    <meta property="og:site_name" content="GoonHub" />
    <meta property="og:type" content="%s" />
    %s
    %s
    <meta name="twitter:card" content="summary_large_image" />
    <meta name="twitter:title" content="%s" />
    <meta name="theme-color" content="#050505" />
    <meta http-equiv="refresh" content="0;url=%s" />
    <title>%s - GoonHub</title>
</head>
<body></body>
</html>`,
		safeTitle, safeURL, ogType,
		imageTags, descTags,
		safeTitle, safeURL, safeTitle,
	)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(body))
}
