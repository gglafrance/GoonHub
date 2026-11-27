package response

import (
	"fmt"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// Profile pages (docs/PROFILES.md). Scenes are SceneListItem redacted with the viewer's file
// visibility; markers use the marker widget shape (versioned thumbnails).

// ProfileBannerURL is the URL of a banner image.
func ProfileBannerURL(b *core.ProfileBanner, ownerID uint, size string) string {
	if b == nil {
		return ""
	}
	if b.Type == data.ProfileBannerScene {
		return fmt.Sprintf("/thumbnails/%d?size=lg", b.SceneID)
	}
	return fmt.Sprintf("/api/v1/profiles/banner/%d/%s?v=%d", ownerID, size, b.Version)
}

// ProfileBannerResponse is the banner of a profile.
type ProfileBannerResponse struct {
	Type    string `json:"type"`
	URL     string `json:"url"`
	URLMd   string `json:"url_md"`
	SceneID *uint  `json:"scene_id,omitempty"`
}

func newProfileBannerResponse(b *core.ProfileBanner, ownerID uint) *ProfileBannerResponse {
	if b == nil {
		return nil
	}
	out := &ProfileBannerResponse{Type: b.Type, URL: ProfileBannerURL(b, ownerID, "lg"), URLMd: ProfileBannerURL(b, ownerID, "md")}
	if b.Type == data.ProfileBannerScene {
		id := b.SceneID
		out.SceneID = &id
	}
	return out
}

// ProfileSceneResponse is a scene of a block with its figure.
type ProfileSceneResponse struct {
	Scene SceneListItem `json:"scene"`
	At    *time.Time    `json:"at,omitempty"`
	Views *int64        `json:"views,omitempty"`
	// Rating is the owner's rating (ratings list)
	Rating *float64 `json:"rating,omitempty"`
	// JizzCount is the owner's O count (jizz block and list)
	JizzCount *int64 `json:"jizz_count,omitempty"`
	// Figures are the owner's figures of a custom list scene (the kinds the list shows and
	// the viewer may see)
	Figures *ProfileSceneFiguresResponse `json:"figures,omitempty"`
}

// ProfileSceneFiguresResponse are the owner's figures of a custom list scene; absent fields:
// not shown, or no such interaction.
type ProfileSceneFiguresResponse struct {
	JizzCount     *int64     `json:"jizz_count,omitempty"`
	LastJizzAt    *time.Time `json:"last_jizz_at,omitempty"`
	Rating        *float64   `json:"rating,omitempty"`
	LikedAt       *time.Time `json:"liked_at,omitempty"`
	Views         *int64     `json:"views,omitempty"`
	LastWatchedAt *time.Time `json:"last_watched_at,omitempty"`
}

func newProfileSceneFiguresResponse(f *core.ProfileSceneFigures) *ProfileSceneFiguresResponse {
	if f == nil {
		return nil
	}
	out := &ProfileSceneFiguresResponse{LastJizzAt: f.LastJizzAt, Rating: f.Rating, LikedAt: f.LikedAt, LastWatchedAt: f.LastWatchedAt}
	if f.JizzCount > 0 {
		v := f.JizzCount
		out.JizzCount = &v
	}
	if f.Views > 0 {
		v := f.Views
		out.Views = &v
	}
	return out
}

// NewProfileSceneResponse converts a scene of a block or list with its figures.
func NewProfileSceneResponse(s core.ProfileSceneItem, item SceneListItem) ProfileSceneResponse {
	out := ProfileSceneResponse{Scene: item, At: s.At, Rating: s.Rating, Figures: newProfileSceneFiguresResponse(s.Figures)}
	if s.Views > 0 {
		v := s.Views
		out.Views = &v
	}
	if s.JizzCount > 0 {
		v := s.JizzCount
		out.JizzCount = &v
	}
	return out
}

// ProfileListsResponse tells which full lists of a profile the viewer may open.
type ProfileListsResponse struct {
	Liked   bool `json:"liked"`
	Ratings bool `json:"ratings"`
	Jizz    bool `json:"jizz"`
}

// ProfileBlockResponse is a block of a profile as the viewer gets it. A locked block (the
// owner's placeholder for viewers who may not see it) has no data.
type ProfileBlockResponse struct {
	ID          string                 `json:"id,omitempty"`
	Type        string                 `json:"type"`
	Label       string                 `json:"label"`
	PrivacyItem string                 `json:"privacy_item"`
	Config      map[string]any         `json:"config"`
	Locked      bool                   `json:"locked"`
	Failed      bool                   `json:"failed,omitempty"`
	Empty       bool                   `json:"empty"`
	Scenes      []ProfileSceneResponse `json:"scenes,omitempty"`
	Markers     []data.WidgetMarker    `json:"markers,omitempty"`
	Widget      any                    `json:"widget,omitempty"`
	Total       *int64                 `json:"total,omitempty"`
}

// ProfileResponse is GET /profiles/users/:username: the social summary (user, relationship,
// visibility, counts) and, when the profile is visible, the bio, the banner and the blocks.
type ProfileResponse struct {
	SocialProfileResponse
	Bio    string                 `json:"bio"`
	Banner *ProfileBannerResponse `json:"banner"`
	Blocks []ProfileBlockResponse `json:"blocks"`
	Lists  ProfileListsResponse   `json:"lists"`
	// Tabs the viewer may open, the served tab and its sections
	Tabs       []ProfileTabResponse     `json:"tabs"`
	Tab        string                   `json:"tab"`
	TabMissing bool                     `json:"tab_missing,omitempty"`
	Sections   []ProfileSectionResponse `json:"sections"`
}

// ProfileTabResponse is a tab of a profile as the viewer lists it.
type ProfileTabResponse struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Icon  string `json:"icon,omitempty"`
	Slug  string `json:"slug,omitempty"`
}

// ProfileSectionResponse is a section of the served tab.
type ProfileSectionResponse struct {
	ID       string                   `json:"id"`
	Template string                   `json:"template"`
	Columns  [][]ProfileBlockResponse `json:"columns"`
}

func newProfileBlockResponse(b core.ProfileBlockView, vis FileVisibility) ProfileBlockResponse {
	out := ProfileBlockResponse{
		ID:          b.ID,
		Type:        b.Spec.Type,
		Label:       b.Spec.Label,
		PrivacyItem: b.Spec.PrivacyItem,
		Config:      map[string]any(b.Config),
		Locked:      b.Locked,
		Failed:      b.Failed,
	}
	if out.Config == nil {
		out.Config = map[string]any{}
	}
	if b.Data == nil {
		return out
	}
	out.Empty = b.Data.Empty
	if b.Data.Title != "" {
		out.Label = b.Data.Title
	}
	out.Widget = b.Data.Widget
	out.Total = b.Data.Total
	if b.Data.Markers != nil {
		out.Markers = b.Data.Markers
	}
	if b.Data.Scenes != nil {
		out.Scenes = make([]ProfileSceneResponse, len(b.Data.Scenes))
		for i, s := range b.Data.Scenes {
			out.Scenes[i] = NewProfileSceneResponse(s, ToSceneListItem(s.Scene, vis))
		}
	}
	return out
}

func NewProfileResponse(v *core.ProfileView, vis FileVisibility) ProfileResponse {
	out := ProfileResponse{
		SocialProfileResponse: NewSocialProfileResponse(v.Social),
		Bio:                   v.Bio,
		Banner:                newProfileBannerResponse(v.Banner, v.Social.User.ID),
		Blocks:                make([]ProfileBlockResponse, len(v.Blocks)),
		Lists:                 ProfileListsResponse{Liked: v.Lists.Liked, Ratings: v.Lists.Ratings, Jizz: v.Lists.Jizz},
	}
	for i, b := range v.Blocks {
		out.Blocks[i] = newProfileBlockResponse(b, vis)
	}
	out.Tabs = make([]ProfileTabResponse, len(v.Tabs))
	for i, t := range v.Tabs {
		out.Tabs[i] = ProfileTabResponse{ID: t.ID, Kind: t.Kind, Title: t.Title, Icon: t.Icon, Slug: t.Slug}
	}
	out.Tab, out.TabMissing = v.Tab, v.TabMissing
	out.Sections = make([]ProfileSectionResponse, len(v.Sections))
	for i, sec := range v.Sections {
		sr := ProfileSectionResponse{ID: sec.ID, Template: sec.Template, Columns: make([][]ProfileBlockResponse, len(sec.Columns))}
		for ci, col := range sec.Columns {
			sr.Columns[ci] = make([]ProfileBlockResponse, len(col))
			for bi, b := range col {
				sr.Columns[ci][bi] = newProfileBlockResponse(b, vis)
			}
		}
		out.Sections[i] = sr
	}
	return out
}

// ProfileBlockOptionResponse describes an option of a block type.
type ProfileBlockOptionResponse struct {
	Key     string               `json:"key"`
	Label   string               `json:"label"`
	Kind    string               `json:"kind"`
	Min     int                  `json:"min,omitempty"`
	Max     int                  `json:"max,omitempty"`
	Default any                  `json:"default"`
	Choices []PrivacyOptionLabel `json:"choices,omitempty"`
}

// PrivacyOptionLabel is a value and its label.
type PrivacyOptionLabel struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ProfileBlockTypeResponse describes a block type of the library.
type ProfileBlockTypeResponse struct {
	Type             string                       `json:"type"`
	Label            string                       `json:"label"`
	Description      string                       `json:"description"`
	PrivacyItem      string                       `json:"privacy_item"`
	ViewerPermission string                       `json:"viewer_permission,omitempty"`
	Options          []ProfileBlockOptionResponse `json:"options"`
	// Multi: placed as many times as wanted, each configured (custom lists)
	Multi bool `json:"multi,omitempty"`
}

// NewProfileBlockTypesResponse lists the registered block types.
func NewProfileBlockTypesResponse() []ProfileBlockTypeResponse {
	specs := core.ProfileBlockSpecs()
	out := make([]ProfileBlockTypeResponse, len(specs))
	for i, spec := range specs {
		t := ProfileBlockTypeResponse{
			Type:             spec.Type,
			Label:            spec.Label,
			Description:      spec.Description,
			PrivacyItem:      spec.PrivacyItem,
			ViewerPermission: spec.ViewerPermission,
			Multi:            spec.Multi,
			Options:          make([]ProfileBlockOptionResponse, len(spec.Options)),
		}
		for j, opt := range spec.Options {
			o := ProfileBlockOptionResponse{Key: opt.Key, Label: opt.Label, Kind: string(opt.Kind)}
			if opt.Kind == core.ProfileOptionInt {
				o.Min, o.Max, o.Default = opt.Min, opt.Max, opt.Default
			} else {
				o.Default = opt.DefaultChoice
				for _, c := range opt.Choices {
					o.Choices = append(o.Choices, PrivacyOptionLabel{Value: c.Value, Label: c.Label})
				}
			}
			t.Options[j] = o
		}
		out[i] = t
	}
	return out
}

// ProfileLayoutResponse is the layout of a profile (PUT /profiles/me/layout).
type ProfileLayoutResponse struct {
	Blocks []data.ProfileBlockConfig `json:"blocks"`
}

// ProfileLimitsResponse are the limits the edit UI shows.
type ProfileLimitsResponse struct {
	MaxShowcaseItems     int   `json:"max_showcase_items"`
	BioMaxLength         int   `json:"bio_max_length"`
	DisplayNameMaxLength int   `json:"display_name_max_length"`
	MaxUploadBytes       int64 `json:"max_upload_bytes"`
	MaxTabs              int   `json:"max_tabs"`
	MaxSections          int   `json:"max_sections"`
	MaxBlocksPerTab      int   `json:"max_blocks_per_tab"`
	MaxLists             int   `json:"max_lists"`
}

// ProfileEditBannerResponse is the banner as its owner edits it.
type ProfileEditBannerResponse struct {
	Type    string `json:"type"`
	SceneID *uint  `json:"scene_id,omitempty"`
	Version int64  `json:"version"`
	URL     string `json:"url,omitempty"`
}

// ProfileEditResponse is GET /profiles/me: everything the owner edits.
type ProfileEditResponse struct {
	User            SocialUserResponse         `json:"user"`
	DisplayName     string                     `json:"display_name"`
	Bio             string                     `json:"bio"`
	AvatarVersion   int64                      `json:"avatar_version"`
	Banner          ProfileEditBannerResponse  `json:"banner"`
	Layout          []data.ProfileBlockConfig  `json:"layout"`
	Tabs            data.ProfileTabs           `json:"tabs"`
	Lists           []CustomListResponse       `json:"lists"`
	Templates       []SectionTemplateResponse  `json:"section_templates"`
	FavoriteScenes  []SceneListItem            `json:"favorite_scenes"`
	FavoriteMarkers []data.WidgetMarker        `json:"favorite_markers"`
	BlockTypes      []ProfileBlockTypeResponse `json:"block_types"`
	Privacy         PrivacySettingsResponse    `json:"privacy"`
	Limits          ProfileLimitsResponse      `json:"limits"`
}

// NewProfileLimitsResponse returns the limits of the configuration.
func NewProfileLimitsResponse(cfg config.ProfilesConfig) ProfileLimitsResponse {
	return ProfileLimitsResponse{
		MaxShowcaseItems:     cfg.MaxShowcaseItems,
		BioMaxLength:         cfg.BioMaxLength,
		DisplayNameMaxLength: cfg.DisplayNameMaxLength,
		MaxUploadBytes:       cfg.MaxUploadBytes,
		MaxTabs:              cfg.MaxTabs,
		MaxSections:          cfg.MaxSections,
		MaxBlocksPerTab:      cfg.MaxBlocksPerTab,
		MaxLists:             cfg.MaxLists,
	}
}

func NewProfileEditResponse(s *core.ProfileEditState, cfg config.ProfilesConfig, vis FileVisibility) ProfileEditResponse {
	p := s.Profile
	banner := ProfileEditBannerResponse{Type: p.BannerType, SceneID: p.BannerSceneID, Version: p.BannerVersion}
	switch {
	case p.BannerType == data.ProfileBannerUpload && p.BannerVersion > 0:
		banner.URL = ProfileBannerURL(&core.ProfileBanner{Type: data.ProfileBannerUpload, Version: p.BannerVersion}, p.UserID, "md")
	case p.BannerType == data.ProfileBannerScene && p.BannerSceneID != nil:
		banner.URL = ProfileBannerURL(&core.ProfileBanner{Type: data.ProfileBannerScene, SceneID: *p.BannerSceneID}, p.UserID, "md")
	}
	return ProfileEditResponse{
		User:            NewSocialUserResponse(s.User),
		DisplayName:     p.DisplayName,
		Bio:             p.Bio,
		AvatarVersion:   p.AvatarVersion,
		Banner:          banner,
		Layout:          s.Layout,
		Tabs:            s.Tabs,
		Lists:           NewCustomListsResponse(s.Lists),
		Templates:       NewSectionTemplatesResponse(),
		FavoriteScenes:  ToSceneListItems(s.FavoriteScenes, vis),
		FavoriteMarkers: s.FavoriteMarkers,
		BlockTypes:      NewProfileBlockTypesResponse(),
		Privacy:         NewPrivacySettingsResponse(s.Privacy),
		Limits:          NewProfileLimitsResponse(cfg),
	}
}

// ProfileListResponse is a page of a full list of a profile (liked, rated, jizz videos): the
// scenes with the owner's figures, the sort used, and the viewer's own card sidecars (ratings,
// likes, jizz_counts, watched maps, as on the scene lists, when card_fields asks for them).
type ProfileListResponse struct {
	Data       []ProfileSceneResponse `json:"data"`
	Pagination Pagination             `json:"pagination"`
	Sort       string                 `json:"sort"`
	CardSidecars
}
