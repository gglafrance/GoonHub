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
	Weight          string `json:"weight,omitempty"`          // e.g. "50kg"
	Height          string `json:"height,omitempty"`          // e.g. "160cm"
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

type pornDBSearchResponse struct {
	Data []PornDBPerformer `json:"data"`
}

type pornDBPerformerResponse struct {
	Data pornDBPerformerRaw `json:"data"`
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
