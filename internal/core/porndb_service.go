package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"go.uber.org/zap"
)

const pornDBBaseURL = "https://api.theporndb.net"

// PornDBPerformer represents a performer from search results
type PornDBPerformer struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Image string `json:"image,omitempty"`
	Bio   string `json:"bio,omitempty"`
}

// pornDBPerformerExtras contains the nested extras data from PornDB API
type pornDBPerformerExtras struct {
	Gender          string `json:"gender,omitempty"`
	Birthday        string `json:"birthday,omitempty"`
	Deathday        string `json:"deathday,omitempty"`
	Birthplace      string `json:"birthplace,omitempty"`
	Astrology       string `json:"astrology,omitempty"`
	Ethnicity       string `json:"ethnicity,omitempty"`
	Nationality     string `json:"nationality,omitempty"`
	HairColour      string `json:"hair_colour,omitempty"`
	EyeColour       string `json:"eye_colour,omitempty"`
	Weight          string `json:"weight,omitempty"` // e.g. "50kg"
	Height          string `json:"height,omitempty"` // e.g. "160cm"
	Measurements    string `json:"measurements,omitempty"`
	Cupsize         string `json:"cupsize,omitempty"`
	Tattoos         string `json:"tattoos,omitempty"`
	Piercings       string `json:"piercings,omitempty"`
	CareerStartYear *int   `json:"career_start_year,omitempty"`
	CareerEndYear   *int   `json:"career_end_year,omitempty"`
	FakeBoobs       *bool  `json:"fake_boobs,omitempty"`
	SameSexOnly     *bool  `json:"same_sex_only,omitempty"`
}

// pornDBPerformerRaw is the raw API response structure
type pornDBPerformerRaw struct {
	ID     string                 `json:"id"`
	Slug   string                 `json:"slug"`
	Name   string                 `json:"name"`
	Image  string                 `json:"image,omitempty"`
	Bio    string                 `json:"bio,omitempty"`
	Extras *pornDBPerformerExtras `json:"extras,omitempty"`
}

// PornDBPerformerDetails is the flattened response we send to the frontend
type PornDBPerformerDetails struct {
	ID              string `json:"id"`
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Image           string `json:"image,omitempty"`
	Bio             string `json:"bio,omitempty"`
	Gender          string `json:"gender,omitempty"`
	Birthday        string `json:"birthday,omitempty"`
	Deathday        string `json:"deathday,omitempty"`
	Birthplace      string `json:"birthplace,omitempty"`
	Astrology       string `json:"astrology,omitempty"`
	Ethnicity       string `json:"ethnicity,omitempty"`
	Nationality     string `json:"nationality,omitempty"`
	HairColour      string `json:"hair_colour,omitempty"`
	EyeColour       string `json:"eye_colour,omitempty"`
	Height          *int   `json:"height,omitempty"`
	Weight          *int   `json:"weight,omitempty"`
	Measurements    string `json:"measurements,omitempty"`
	Cupsize         string `json:"cupsize,omitempty"`
	Tattoos         string `json:"tattoos,omitempty"`
	Piercings       string `json:"piercings,omitempty"`
	CareerStartYear *int   `json:"career_start_year,omitempty"`
	CareerEndYear   *int   `json:"career_end_year,omitempty"`
	FakeBoobs       *bool  `json:"fake_boobs,omitempty"`
	SameSexOnly     *bool  `json:"same_sex_only,omitempty"`
}

// PornDBScene represents a scene from ThePornDB
type PornDBScene struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Date        string                 `json:"date,omitempty"`
	Duration    int                    `json:"duration,omitempty"`
	Image       string                 `json:"image,omitempty"`
	Poster      string                 `json:"poster,omitempty"`
	Site        *PornDBSite            `json:"site,omitempty"`
	Performers  []PornDBScenePerformer `json:"performers,omitempty"`
	Tags        []PornDBTag            `json:"tags,omitempty"`
	Markers     []PornDBMarker         `json:"markers,omitempty"`
	Parse       string                 `json:"parse,omitempty"`
}

// PornDBSite represents a site/studio from ThePornDB (lightweight for scene responses)
type PornDBSite struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// PornDBSiteDetails represents detailed site information from ThePornDB
type PornDBSiteDetails struct {
	ID          string   `json:"id"`
	UUID        string   `json:"uuid,omitempty"`
	Slug        string   `json:"slug,omitempty"`
	Name        string   `json:"name"`
	ShortName   string   `json:"short_name,omitempty"`
	URL         string   `json:"url,omitempty"`
	Description string   `json:"description,omitempty"`
	Rating      *float64 `json:"rating,omitempty"`
	Logo        string   `json:"logo,omitempty"`
	Favicon     string   `json:"favicon,omitempty"`
	Poster      string   `json:"poster,omitempty"`
	Network     string   `json:"network,omitempty"`
	Parent      string   `json:"parent,omitempty"`
	NetworkID   string   `json:"network_id,omitempty"`
	ParentID    string   `json:"parent_id,omitempty"`
}

// PornDBScenePerformer represents a performer in a scene
type PornDBScenePerformer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image,omitempty"`
}

// PornDBTag represents a tag from ThePornDB
type PornDBTag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// PornDBMarker represents a marker/chapter from ThePornDB
type PornDBMarker struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	StartTime int    `json:"start_time"`
	EndTime   *int   `json:"end_time,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type pornDBSearchResponse struct {
	Data []PornDBPerformer `json:"data"`
}

type pornDBPerformerResponse struct {
	Data pornDBPerformerRaw `json:"data"`
}

// pornDBSceneRaw is the raw API response structure for a scene
type pornDBSceneRaw struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Date        string `json:"date,omitempty"`
	Duration    int    `json:"duration,omitempty"`
	Image       string `json:"image,omitempty"`
	Poster      string `json:"poster,omitempty"`
	Background  *struct {
		Full   string `json:"full,omitempty"`
		Large  string `json:"large,omitempty"`
		Medium string `json:"medium,omitempty"`
		Small  string `json:"small,omitempty"`
	} `json:"background,omitempty"`
	Site *struct {
		Name string `json:"name"`
		URL  string `json:"url,omitempty"`
	} `json:"site,omitempty"`
	Performers []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Image string `json:"image,omitempty"`
	} `json:"performers,omitempty"`
	Tags []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"tags,omitempty"`
	Markers []struct {
		ID        int    `json:"id"`
		Title     string `json:"title"`
		StartTime int    `json:"start_time"`
		EndTime   *int   `json:"end_time,omitempty"`
		CreatedAt string `json:"created_at,omitempty"`
	} `json:"markers,omitempty"`
}

type pornDBSceneSearchResponse struct {
	Data []pornDBSceneRaw `json:"data"`
}

type pornDBSceneResponse struct {
	Data pornDBSceneRaw `json:"data"`
}

// PornDBService handles communication with ThePornDB API
type PornDBService struct {
	apiKey string
	client *http.Client
	logger *zap.Logger
}

// NewPornDBService creates a new PornDB service
func NewPornDBService(apiKey string, logger *zap.Logger) *PornDBService {
	return &PornDBService{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// IsConfigured returns true if the API key is configured
func (s *PornDBService) IsConfigured() bool {
	return s.apiKey != ""
}

// SearchPerformers searches for performers by name
func (s *PornDBService) SearchPerformers(query string) ([]PornDBPerformer, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("PornDB API key is not configured")
	}

	params := url.Values{}
	params.Set("q", query)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/performers?%s", pornDBBaseURL, params.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Warn("PornDB search failed",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("PornDB API returned status %d", resp.StatusCode)
	}

	var result pornDBSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

// parseNumericValue extracts a number from a string like "160cm" or "50kg"
func parseNumericValue(s string) *int {
	if s == "" {
		return nil
	}
	re := regexp.MustCompile(`(\d+)`)
	matches := re.FindStringSubmatch(s)
	if len(matches) < 2 {
		return nil
	}
	val, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil
	}
	return &val
}

// GetPerformerDetails fetches detailed information about a performer
func (s *PornDBService) GetPerformerDetails(id string) (*PornDBPerformerDetails, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("PornDB API key is not configured")
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/performers/%s", pornDBBaseURL, id), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn("PornDB get performer failed",
			zap.String("id", id),
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("PornDB API returned status %d", resp.StatusCode)
	}

	var result pornDBPerformerResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Flatten the response for the frontend
	raw := result.Data
	details := &PornDBPerformerDetails{
		ID:    raw.ID,
		Slug:  raw.Slug,
		Name:  raw.Name,
		Image: raw.Image,
		Bio:   raw.Bio,
	}

	// Copy fields from extras if present
	if raw.Extras != nil {
		extras := raw.Extras
		details.Gender = extras.Gender
		details.Birthday = extras.Birthday
		details.Deathday = extras.Deathday
		details.Birthplace = extras.Birthplace
		details.Astrology = extras.Astrology
		details.Ethnicity = extras.Ethnicity
		details.Nationality = extras.Nationality
		details.HairColour = extras.HairColour
		details.EyeColour = extras.EyeColour
		details.Measurements = extras.Measurements
		details.Cupsize = extras.Cupsize
		details.Tattoos = extras.Tattoos
		details.Piercings = extras.Piercings
		details.CareerStartYear = extras.CareerStartYear
		details.CareerEndYear = extras.CareerEndYear
		details.FakeBoobs = extras.FakeBoobs
		details.SameSexOnly = extras.SameSexOnly

		// Parse height and weight from strings like "160cm" and "50kg"
		details.Height = parseNumericValue(extras.Height)
		details.Weight = parseNumericValue(extras.Weight)
	}

	return details, nil
}

// convertRawSceneToScene converts a raw scene response to a PornDBScene
func convertRawSceneToScene(raw pornDBSceneRaw) PornDBScene {
	image := raw.Image
	if raw.Background != nil && raw.Background.Large != "" {
		image = raw.Background.Large
	}

	scene := PornDBScene{
		ID:          raw.ID,
		Title:       raw.Title,
		Description: raw.Description,
		Date:        raw.Date,
		Duration:    raw.Duration,
		Image:       image,
		Poster:      raw.Poster,
	}

	if raw.Site != nil {
		scene.Site = &PornDBSite{
			Name: raw.Site.Name,
			URL:  raw.Site.URL,
		}
	}

	for _, p := range raw.Performers {
		scene.Performers = append(scene.Performers, PornDBScenePerformer{
			ID:    p.ID,
			Name:  p.Name,
			Image: p.Image,
		})
	}

	for _, t := range raw.Tags {
		scene.Tags = append(scene.Tags, PornDBTag{
			ID:   t.ID,
			Name: t.Name,
		})
	}

	for _, m := range raw.Markers {
		scene.Markers = append(scene.Markers, PornDBMarker{
			ID:        m.ID,
			Title:     m.Title,
			StartTime: m.StartTime,
			EndTime:   m.EndTime,
			CreatedAt: m.CreatedAt,
		})
	}

	return scene
}

// SceneSearchOptions contains optional search parameters for scene search
type SceneSearchOptions struct {
	Query string // General text search (q)
	Title string // Scene title
	Year  int    // Release year
	Site  string // Studio/site name
}

// IsEmpty returns true if no search parameters are set
func (o SceneSearchOptions) IsEmpty() bool {
	return o.Query == "" && o.Title == "" && o.Year == 0 && o.Site == ""
}

// SearchScenes searches for scenes with optional filters
func (s *PornDBService) SearchScenes(opts SceneSearchOptions) ([]PornDBScene, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("PornDB API key is not configured")
	}

	params := url.Values{}
	if opts.Query != "" {
		params.Set("q", opts.Query)
	}
	if opts.Title != "" {
		params.Set("parse", opts.Title)
	}
	if opts.Year > 0 {
		params.Set("year", strconv.Itoa(opts.Year))
	}
	if opts.Site != "" {
		params.Set("site", opts.Site)
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/scenes?%s", pornDBBaseURL, params.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Warn("PornDB scene search failed",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("PornDB API returned status %d", resp.StatusCode)
	}

	var result pornDBSceneSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	scenes := make([]PornDBScene, 0, len(result.Data))
	for _, raw := range result.Data {
		scenes = append(scenes, convertRawSceneToScene(raw))
	}

	return scenes, nil
}

// GetSceneDetails fetches detailed information about a scene
func (s *PornDBService) GetSceneDetails(id string) (*PornDBScene, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("PornDB API key is not configured")
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/scenes/%s", pornDBBaseURL, id), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn("PornDB get scene failed",
			zap.String("id", id),
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("PornDB API returned status %d", resp.StatusCode)
	}

	var result pornDBSceneResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	scene := convertRawSceneToScene(result.Data)
	return &scene, nil
}

// pornDBSiteRaw is the raw API response structure for a site
type pornDBSiteRaw struct {
	ID        json.Number `json:"id"`
	UUID      string      `json:"uuid,omitempty"`
	Slug      string      `json:"slug,omitempty"`
	Name      string      `json:"name"`
	ShortName string      `json:"short_name,omitempty"`
	URL       string      `json:"url,omitempty"`
	Bio       string      `json:"bio,omitempty"`
	Rating    json.Number `json:"rating,omitempty"`
	Logo      string      `json:"logo,omitempty"`
	Favicon   string      `json:"favicon,omitempty"`
	Poster    string      `json:"poster,omitempty"`
	Network   *struct {
		ID   json.Number `json:"id"`
		Name string      `json:"name"`
	} `json:"network,omitempty"`
	Parent *struct {
		ID   json.Number `json:"id"`
		Name string      `json:"name"`
	} `json:"parent,omitempty"`
}

type pornDBSiteSearchResponse struct {
	Data []pornDBSiteRaw `json:"data"`
}

type pornDBSiteResponse struct {
	Data pornDBSiteRaw `json:"data"`
}

func convertRawSiteToSiteDetails(raw pornDBSiteRaw) PornDBSiteDetails {
	site := PornDBSiteDetails{
		ID:          string(raw.ID),
		UUID:        raw.UUID,
		Slug:        raw.Slug,
		Name:        raw.Name,
		ShortName:   raw.ShortName,
		URL:         raw.URL,
		Description: raw.Bio,
		Logo:        raw.Logo,
		Favicon:     raw.Favicon,
		Poster:      raw.Poster,
	}

	if raw.Rating != "" {
		if rating, err := raw.Rating.Float64(); err == nil {
			site.Rating = &rating
		}
	}

	if raw.Network != nil {
		site.Network = raw.Network.Name
		site.NetworkID = string(raw.Network.ID)
	}

	if raw.Parent != nil {
		site.Parent = raw.Parent.Name
		site.ParentID = string(raw.Parent.ID)
	}

	return site
}

// SearchSites searches for sites/studios by name
func (s *PornDBService) SearchSites(query string) ([]PornDBSiteDetails, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("PornDB API key is not configured")
	}

	params := url.Values{}
	params.Set("q", query)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/sites?%s", pornDBBaseURL, params.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Warn("PornDB site search failed",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("PornDB API returned status %d", resp.StatusCode)
	}

	var result pornDBSiteSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	sites := make([]PornDBSiteDetails, 0, len(result.Data))
	for _, raw := range result.Data {
		sites = append(sites, convertRawSiteToSiteDetails(raw))
	}

	return sites, nil
}

// GetSiteDetails fetches detailed information about a site
func (s *PornDBService) GetSiteDetails(id string) (*PornDBSiteDetails, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("PornDB API key is not configured")
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/sites/%s", pornDBBaseURL, id), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn("PornDB get site failed",
			zap.String("id", id),
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("PornDB API returned status %d", resp.StatusCode)
	}

	var result pornDBSiteResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	site := convertRawSiteToSiteDetails(result.Data)
	return &site, nil
}
