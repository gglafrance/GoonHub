package core

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"goonhub/internal/data"
)

// Profile blocks (docs/PROFILES.md): the sections an owner composes their profile page from.
// Each block type lives in its own profile_block_<type>.go file and registers itself from
// init():
//
//	func init() {
//		registerProfileBlock(ProfileBlockSpec{Type: "my_block", PrivacyItem: PrivacyX, Fetch: fetchMyBlock, ...})
//	}
//
// The service checks, before Fetch runs, that the viewer may see the block: the block's
// privacy item (and the profile item) for the owner, and the viewer's permission when the
// block needs one. Fetch never sees a viewer who may not see the block, and only reads data
// of the owner. Scenes are returned as data.Scene and redacted by the API layer with the
// viewer's file visibility.

// ProfileBlockOptionKind is the kind of a block option.
type ProfileBlockOptionKind string

const (
	// ProfileOptionInt is a whole number between Min and Max.
	ProfileOptionInt ProfileBlockOptionKind = "int"
	// ProfileOptionChoice is one of Choices.
	ProfileOptionChoice ProfileBlockOptionKind = "choice"
)

// ProfileBlockChoice is a value of a choice option.
type ProfileBlockChoice struct {
	Value string
	Label string
}

// ProfileBlockOption describes an option of a block, for the validation and the edit UI.
type ProfileBlockOption struct {
	Key   string
	Label string
	Kind  ProfileBlockOptionKind
	// Int options
	Min, Max, Default int
	// Choice options
	Choices       []ProfileBlockChoice
	DefaultChoice string
}

// ProfileBlockSpec describes a block type.
type ProfileBlockSpec struct {
	Type        string
	Label       string
	Description string
	// PrivacyItem is the privacy item (privacy_catalog.go) that decides who sees the block.
	PrivacyItem string
	// ViewerPermission is the permission the viewer needs to see the block (empty: none),
	// e.g. scenes:view for the blocks showing scenes.
	ViewerPermission string
	Options          []ProfileBlockOption
	// DefaultVisible: shown in the default layout.
	DefaultVisible bool
	// Order is the position in the default layout and in the block library.
	Order int
	// Multi: a block type placed as many times as the owner wants, each one configured (the
	// custom lists); never part of the default layout nor appended to older layouts.
	Multi bool
	Fetch func(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error)
}

// ProfileBlockDeps gives the blocks access to the data they read.
type ProfileBlockDeps struct {
	Repo     data.ProfileRepository
	Social   data.SocialRepository
	Markers  data.MarkerRepository
	Homepage *HomepageService
	// RunList runs a custom list for a viewer (ProfileService.runList)
	RunList func(ctx context.Context, ownerID uint, viewer ProfileViewer, f data.Filters, page, limit int) ([]data.Scene, int64, error)
	// Now returns the current time (tests).
	Now func() time.Time
}

// ProfileBlockRequest is what a block is fetched for.
type ProfileBlockRequest struct {
	OwnerID uint
	Viewer  ProfileViewer
	Config  ProfileBlockValues
	// List is the custom list of a custom_list block (already checked for the viewer)
	List *ResolvedProfileList
}

// ProfileBlockValues are the resolved options of a block (defaults filled in).
type ProfileBlockValues map[string]any

// Int returns an int option.
func (v ProfileBlockValues) Int(key string) int {
	n, _ := v[key].(int)
	return n
}

// String returns a choice option.
func (v ProfileBlockValues) String(key string) string {
	s, _ := v[key].(string)
	return s
}

// ProfileSceneItem is a scene of a block or list with its figure (watched, liked, rated or
// last jizzed at, views, the owner's rating, the owner's O count).
type ProfileSceneItem struct {
	Scene     data.Scene
	At        *time.Time
	Views     int64
	Rating    *float64
	JizzCount int64
}

// ProfileBlockData is the content of a block. Scenes are redacted by the API layer; Widget is
// any JSON payload for the frontend component of the block type.
type ProfileBlockData struct {
	Scenes  []ProfileSceneItem
	Markers []data.WidgetMarker
	Widget  any
	// Total is the full count when the block shows a part (liked scenes, playlists, friends)
	Total *int64
	// Empty: nothing to show (the page hides empty blocks from other viewers)
	Empty bool
	// Title replaces the label of the block type (custom lists)
	Title string
}

var profileBlocks = map[string]ProfileBlockSpec{}

// registerProfileBlock registers a block type (call from init()).
func registerProfileBlock(spec ProfileBlockSpec) {
	if spec.Type == "" || spec.Fetch == nil {
		panic(fmt.Sprintf("profile block %q has no type or no Fetch function", spec.Type))
	}
	if _, ok := privacyItemByKey(spec.PrivacyItem); !ok {
		panic(fmt.Sprintf("profile block %q: unknown privacy item %q", spec.Type, spec.PrivacyItem))
	}
	if _, exists := profileBlocks[spec.Type]; exists {
		panic(fmt.Sprintf("profile block %q registered twice", spec.Type))
	}
	profileBlocks[spec.Type] = spec
}

// ProfileBlockSpecs lists the block types in library order.
func ProfileBlockSpecs() []ProfileBlockSpec {
	out := make([]ProfileBlockSpec, 0, len(profileBlocks))
	for _, spec := range profileBlocks {
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Type < out[j].Type
	})
	return out
}

// ProfileBlockSpecFor returns the spec of a block type.
func ProfileBlockSpecFor(blockType string) (ProfileBlockSpec, bool) {
	spec, ok := profileBlocks[blockType]
	return spec, ok
}

// DefaultProfileLayout is the layout of a user who never customized theirs: every block type
// in library order, the DefaultVisible ones shown.
func DefaultProfileLayout() data.ProfileLayout {
	specs := ProfileBlockSpecs()
	out := make(data.ProfileLayout, 0, len(specs))
	for _, spec := range specs {
		if spec.Multi {
			continue
		}
		out = append(out, data.ProfileBlockConfig{Type: spec.Type, Visible: spec.DefaultVisible, Config: map[string]any{}})
	}
	return out
}

// NormalizeProfileLayout returns a stored layout as served: unknown types, duplicates and
// invalid options dropped, missing types appended hidden (block types added later do not
// appear on customized profiles by themselves), nil = the default layout.
func NormalizeProfileLayout(layout data.ProfileLayout) data.ProfileLayout {
	if layout == nil {
		return DefaultProfileLayout()
	}
	seen := make(map[string]bool, len(layout))
	out := make(data.ProfileLayout, 0, len(profileBlocks))
	for _, b := range layout {
		spec, ok := profileBlocks[b.Type]
		if !ok || seen[b.Type] {
			continue
		}
		seen[b.Type] = true
		cfg := map[string]any{}
		for _, opt := range spec.Options {
			if v, ok := b.Config[opt.Key]; ok {
				if norm, err := opt.normalize(v); err == nil {
					cfg[opt.Key] = norm
				}
			}
		}
		out = append(out, data.ProfileBlockConfig{Type: b.Type, Visible: b.Visible, LockedPlaceholder: b.LockedPlaceholder, Config: cfg})
	}
	for _, spec := range ProfileBlockSpecs() {
		if !seen[spec.Type] && !spec.Multi {
			out = append(out, data.ProfileBlockConfig{Type: spec.Type, Config: map[string]any{}})
		}
	}
	return out
}

// ValidateProfileLayout checks a layout sent by its owner (unknown type, duplicate, unknown
// or invalid option: validation error) and returns it normalized (missing types appended
// hidden, option values in their canonical type).
func ValidateProfileLayout(layout data.ProfileLayout) (data.ProfileLayout, error) {
	seen := make(map[string]bool, len(layout))
	for i, b := range layout {
		spec, ok := profileBlocks[b.Type]
		if !ok {
			return nil, profileLayoutError(fmt.Sprintf("unknown block type %q", b.Type))
		}
		if seen[b.Type] {
			return nil, profileLayoutError(fmt.Sprintf("block %q appears twice", b.Type))
		}
		seen[b.Type] = true
		cfg := make(map[string]any, len(b.Config))
		for key, raw := range b.Config {
			opt, ok := spec.option(key)
			if !ok {
				return nil, profileLayoutError(fmt.Sprintf("block %q has no option %q", b.Type, key))
			}
			v, err := opt.normalize(raw)
			if err != nil {
				return nil, profileLayoutError(fmt.Sprintf("block %q: %v", b.Type, err))
			}
			cfg[key] = v
		}
		layout[i].Config = cfg
	}
	return NormalizeProfileLayout(layout), nil
}

func (s ProfileBlockSpec) option(key string) (ProfileBlockOption, bool) {
	for _, opt := range s.Options {
		if opt.Key == key {
			return opt, true
		}
	}
	return ProfileBlockOption{}, false
}

// resolve fills the defaults of the options of a block.
func (s ProfileBlockSpec) resolve(cfg map[string]any) ProfileBlockValues {
	out := make(ProfileBlockValues, len(s.Options))
	for _, opt := range s.Options {
		if v, ok := cfg[opt.Key]; ok {
			if norm, err := opt.normalize(v); err == nil {
				out[opt.Key] = norm
				continue
			}
		}
		if opt.Kind == ProfileOptionInt {
			out[opt.Key] = opt.Default
		} else {
			out[opt.Key] = opt.DefaultChoice
		}
	}
	return out
}

// normalize checks an option value and returns it in its canonical type (int or string).
func (o ProfileBlockOption) normalize(raw any) (any, error) {
	switch o.Kind {
	case ProfileOptionInt:
		var n float64
		switch v := raw.(type) {
		case float64:
			n = v
		case int:
			n = float64(v)
		case int64:
			n = float64(v)
		default:
			return nil, fmt.Errorf("%s must be a number", o.Key)
		}
		if math.IsNaN(n) || n != math.Trunc(n) || n < float64(o.Min) || n > float64(o.Max) {
			return nil, fmt.Errorf("%s must be a whole number between %d and %d", o.Key, o.Min, o.Max)
		}
		return int(n), nil
	case ProfileOptionChoice:
		v, ok := raw.(string)
		if ok {
			for _, c := range o.Choices {
				if c.Value == v {
					return v, nil
				}
			}
		}
		values := make([]string, len(o.Choices))
		for i, c := range o.Choices {
			values[i] = c.Value
		}
		return nil, fmt.Errorf("%s must be one of %s", o.Key, strings.Join(values, ", "))
	}
	return nil, fmt.Errorf("unknown option kind %q", o.Kind)
}

// --- Shared option and fetch helpers of the blocks ---

// profileCountOption is the "how many items" option of the list blocks.
func profileCountOption(def int) ProfileBlockOption {
	return ProfileBlockOption{Key: "count", Label: "Items", Kind: ProfileOptionInt, Min: 2, Max: 24, Default: def}
}

// Periods of the period options.
const (
	profilePeriodWeek  = "week"
	profilePeriodMonth = "month"
	profilePeriodYear  = "year"
	profilePeriodAll   = "all"
)

func profilePeriodOption(def string) ProfileBlockOption {
	return ProfileBlockOption{Key: "period", Label: "Period", Kind: ProfileOptionChoice, DefaultChoice: def, Choices: []ProfileBlockChoice{
		{profilePeriodWeek, "Last 7 days"}, {profilePeriodMonth, "Last 30 days"}, {profilePeriodYear, "Last 365 days"}, {profilePeriodAll, "All time"},
	}}
}

// profilePeriodSince is the start of a period (zero time: all time).
func profilePeriodSince(period string, now time.Time) time.Time {
	switch period {
	case profilePeriodWeek:
		return now.AddDate(0, 0, -7)
	case profilePeriodMonth:
		return now.AddDate(0, 0, -30)
	case profilePeriodYear:
		return now.AddDate(-1, 0, 0)
	default:
		return time.Time{}
	}
}

func (d *ProfileBlockDeps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// profileSceneItems loads the scenes of refs (live ones, in order) with their figures.
func profileSceneItems(ctx context.Context, deps *ProfileBlockDeps, refs []data.ProfileSceneRef) ([]ProfileSceneItem, error) {
	ids := make([]uint, len(refs))
	for i, r := range refs {
		ids[i] = r.SceneID
	}
	scenes, err := deps.Repo.ScenesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to load scenes: %w", err)
	}
	byID := make(map[uint]data.ProfileSceneRef, len(refs))
	for _, r := range refs {
		byID[r.SceneID] = r
	}
	items := make([]ProfileSceneItem, len(scenes))
	for i, s := range scenes {
		r := byID[s.ID]
		items[i] = ProfileSceneItem{Scene: s, At: r.At, Views: r.Count, Rating: r.Rating}
	}
	return items, nil
}

// asJizzItems moves the count of jizz refs (the O count, not views) to JizzCount.
func asJizzItems(items []ProfileSceneItem) []ProfileSceneItem {
	for i := range items {
		items[i].JizzCount, items[i].Views = items[i].Views, 0
	}
	return items
}

// withMarkerTags adds their tags to markers (best effort: the cards work without them).
func withMarkerTags(deps *ProfileBlockDeps, markers []data.WidgetMarker) []data.WidgetMarker {
	if deps.Markers == nil || len(markers) == 0 {
		return markers
	}
	ids := make([]uint, len(markers))
	for i, m := range markers {
		ids[i] = m.ID
	}
	if tags, err := deps.Markers.GetMarkerTagsMultiple(ids); err == nil {
		for i := range markers {
			markers[i].Tags = tags[markers[i].ID]
		}
	}
	return markers
}

func profileLayoutError(msg string) error {
	return newProfileValidationError("layout", msg)
}
