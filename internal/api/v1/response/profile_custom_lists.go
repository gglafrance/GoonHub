package response

import (
	"time"

	"goonhub/internal/core"
	"goonhub/internal/data"
)

// Custom lists and section templates of the profiles (docs/PROFILES.md, "Custom lists").

// CustomListDependencyResponse is a privacy item a list depends on, with the owner's audience.
type CustomListDependencyResponse struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Audience string `json:"audience"`
}

// CustomListResponse is a list as its owner edits it.
type CustomListResponse struct {
	ID                 uint                           `json:"id"`
	Title              string                         `json:"title"`
	Filters            data.Filters                   `json:"filters"`
	SavedSearchUUID    *string                        `json:"saved_search_uuid"`
	FollowsSavedSearch bool                           `json:"follows_saved_search"`
	Audience           string                         `json:"audience"`
	Dependencies       []CustomListDependencyResponse `json:"dependencies"`
	NeedsSize          bool                           `json:"needs_size"`
	// Figures is the setting (auto, none, jizz, rating, liked, views); AutoFigures what auto
	// shows for the current criteria, main first
	Figures     string    `json:"figures"`
	AutoFigures []string  `json:"auto_figures"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewCustomListResponse(l core.CustomListInfo) CustomListResponse {
	out := CustomListResponse{
		ID: l.List.ID, Title: l.List.Title, Filters: l.Filters, SavedSearchUUID: l.List.SavedSearchUUID,
		FollowsSavedSearch: l.FollowsSavedSearch, Audience: l.List.Audience, NeedsSize: l.NeedsSize,
		Dependencies: make([]CustomListDependencyResponse, len(l.Dependencies)), UpdatedAt: l.List.UpdatedAt,
		Figures: l.List.Figures, AutoFigures: core.AutoListFigures(l.Filters),
	}
	if out.Figures == "" {
		out.Figures = core.ListFiguresAuto
	}
	for i, d := range l.Dependencies {
		out.Dependencies[i] = CustomListDependencyResponse{Key: d.Key, Label: d.Label, Audience: string(d.Audience)}
	}
	return out
}

func NewCustomListsResponse(lists []core.CustomListInfo) []CustomListResponse {
	out := make([]CustomListResponse, len(lists))
	for i, l := range lists {
		out[i] = NewCustomListResponse(l)
	}
	return out
}

// SectionTemplateResponse is a column template of the sections.
type SectionTemplateResponse struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Widths []int  `json:"widths"`
}

func NewSectionTemplatesResponse() []SectionTemplateResponse {
	out := make([]SectionTemplateResponse, len(core.ProfileSectionTemplates))
	for i, t := range core.ProfileSectionTemplates {
		out[i] = SectionTemplateResponse{ID: t.ID, Label: t.Label, Widths: t.Widths}
	}
	return out
}

// CustomListBrowseResponse is a page of a list as a viewer browses it (the viewer's card
// sidecars with card_fields).
type CustomListBrowseResponse struct {
	List       CustomListSummary      `json:"list"`
	Data       []ProfileSceneResponse `json:"data"`
	Pagination Pagination             `json:"pagination"`
	// Figures are the owner's figures on the items, main first (those the viewer may see)
	Figures []string `json:"figures"`
	CardSidecars
}

// CustomListSummary is what a viewer sees of a list: its title (never its criteria).
type CustomListSummary struct {
	ID    uint   `json:"id"`
	Title string `json:"title"`
}
