package core

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"goonhub/internal/data"
)

// Tabs and sections of a profile (docs/PROFILES.md, "Tabs and sections"). A profile has the
// built-in tabs (profile, likes, ratings, jizz; each once, profile always shown) and the
// owner's custom tabs. The profile and custom tabs are made of sections: a column template and
// the blocks of each column. Profiles saved before the tabs (a single list of blocks, the
// layout column) are read as one profile tab holding one full-width section with those blocks,
// so they render unchanged.

// ProfileSectionTemplate is a column template of a section: its columns and their widths (in
// twelfths, for the UI).
type ProfileSectionTemplate struct {
	ID     string
	Label  string
	Widths []int
}

// ProfileSectionTemplates lists the column templates, in the order of the editor.
var ProfileSectionTemplates = []ProfileSectionTemplate{
	{"1", "One column", []int{12}},
	{"1-1", "Two columns", []int{6, 6}},
	{"1-1-1", "Three columns", []int{4, 4, 4}},
	{"2-1", "Wide + sidebar", []int{8, 4}},
	{"1-2", "Sidebar + wide", []int{4, 8}},
}

func sectionTemplate(id string) (ProfileSectionTemplate, bool) {
	for _, t := range ProfileSectionTemplates {
		if t.ID == id {
			return t, true
		}
	}
	return ProfileSectionTemplate{}, false
}

// ProfileTabLimits bound what a profile may hold (config profiles.*).
type ProfileTabLimits struct {
	MaxTabs         int
	MaxSections     int // per tab
	MaxBlocksPerTab int
}

// builtinTabs are the built-in tabs in their default order, with their default title and
// icon (the UI shows them).
var builtinTabs = []data.ProfileTab{
	{ID: data.ProfileTabProfile, Kind: data.ProfileTabProfile, Title: "Profile", Icon: "heroicons:user", Visible: true},
	{ID: data.ProfileTabLikes, Kind: data.ProfileTabLikes, Title: "Likes", Icon: "heroicons:heart", Visible: true},
	{ID: data.ProfileTabRatings, Kind: data.ProfileTabRatings, Title: "Ratings", Icon: "heroicons:star", Visible: true},
	{ID: data.ProfileTabJizz, Kind: data.ProfileTabJizz, Title: "Jizz", Icon: "fluent-emoji-high-contrast:sweat-droplets", Visible: true},
}

func builtinTab(kind string) (data.ProfileTab, bool) {
	for _, t := range builtinTabs {
		if t.Kind == kind {
			return t, true
		}
	}
	return data.ProfileTab{}, false
}

// tabHasSections reports whether a tab kind is made of sections (profile and custom).
func tabHasSections(kind string) bool {
	return kind == data.ProfileTabProfile || kind == data.ProfileTabCustom
}

var (
	profileIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	profileSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	profileIconPattern = regexp.MustCompile(`^[a-z0-9-]+:[a-z0-9-]+$`)
)

const profileTabTitleMax = 40

// ProfileSlug makes a tab slug from a title ("Top O!" -> "top-o"); "" when nothing is left.
func ProfileSlug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if b.Len() > 0 && !dash {
			b.WriteRune('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	return slug
}

// legacyProfileTab is the profile tab of a profile saved before the tabs: one full-width
// section with the blocks of the layout (block ids = their type, unique there).
func legacyProfileTab(layout data.ProfileLayout) data.ProfileTab {
	blocks := NormalizeProfileLayout(layout)
	for i := range blocks {
		blocks[i].ID = blocks[i].Type
	}
	tab, _ := builtinTab(data.ProfileTabProfile)
	tab.Sections = []data.ProfileSection{{ID: "main", Template: "1", Columns: [][]data.ProfileBlockConfig{blocks}}}
	return tab
}

// DefaultProfileTabs are the tabs of a profile that never saved any: the default layout in the
// profile tab, then the list tabs.
func DefaultProfileTabs() data.ProfileTabs {
	return NormalizeProfileTabs(nil, nil)
}

// NormalizeProfileTabs returns the tabs of a profile as served: stored tabs cleaned (unknown
// kinds, templates, block types and invalid options dropped, columns fitted to their template,
// missing built-in tabs appended), or, without stored tabs, the older layout converted.
func NormalizeProfileTabs(tabs data.ProfileTabs, layout data.ProfileLayout) data.ProfileTabs {
	if tabs == nil {
		out := data.ProfileTabs{legacyProfileTab(layout)}
		for _, t := range builtinTabs[1:] {
			out = append(out, t)
		}
		return out
	}
	out := make(data.ProfileTabs, 0, len(tabs)+len(builtinTabs))
	seen := map[string]bool{}
	seenKinds := map[string]bool{}
	for _, t := range tabs {
		if seen[t.ID] || !profileIDPattern.MatchString(t.ID) {
			continue
		}
		if t.Kind != data.ProfileTabCustom {
			def, ok := builtinTab(t.Kind)
			if !ok || seenKinds[t.Kind] {
				continue
			}
			seenKinds[t.Kind] = true
			t.ID, t.Title, t.Icon, t.Slug = def.ID, def.Title, def.Icon, ""
			if t.Kind == data.ProfileTabProfile {
				t.Visible = true
			}
		}
		seen[t.ID] = true
		if tabHasSections(t.Kind) {
			t.Sections = normalizeSections(t.Sections)
		} else {
			t.Sections = nil
		}
		out = append(out, t)
	}
	for _, def := range builtinTabs {
		if !seenKinds[def.Kind] {
			if def.Kind == data.ProfileTabProfile {
				def = legacyProfileTab(nil)
				out = append(data.ProfileTabs{def}, out...)
				continue
			}
			out = append(out, def)
		}
	}
	return out
}

// normalizeSections cleans stored sections (read path, never fails).
func normalizeSections(sections []data.ProfileSection) []data.ProfileSection {
	out := make([]data.ProfileSection, 0, len(sections))
	for _, sec := range sections {
		tpl, ok := sectionTemplate(sec.Template)
		if !ok {
			tpl = ProfileSectionTemplates[0]
		}
		cols := make([][]data.ProfileBlockConfig, len(tpl.Widths))
		for i, col := range sec.Columns {
			target := min(i, len(cols)-1)
			for _, b := range col {
				if nb, ok := normalizeBlock(b); ok {
					cols[target] = append(cols[target], nb)
				}
			}
		}
		for i := range cols {
			if cols[i] == nil {
				cols[i] = []data.ProfileBlockConfig{}
			}
		}
		out = append(out, data.ProfileSection{ID: sec.ID, Template: tpl.ID, Columns: cols})
	}
	return out
}

// normalizeBlock keeps a known block with its valid options.
func normalizeBlock(b data.ProfileBlockConfig) (data.ProfileBlockConfig, bool) {
	spec, ok := profileBlocks[b.Type]
	if !ok {
		return b, false
	}
	cfg := map[string]any{}
	for _, opt := range spec.Options {
		if v, ok := b.Config[opt.Key]; ok {
			if norm, err := opt.normalize(v); err == nil {
				cfg[opt.Key] = norm
			}
		}
	}
	return data.ProfileBlockConfig{ID: b.ID, Type: b.Type, Visible: b.Visible, LockedPlaceholder: b.LockedPlaceholder, Config: cfg}, true
}

func profileTabsError(msg string) error {
	return newProfileValidationError("tabs", msg)
}

// ValidateProfileTabs checks the tabs sent by their owner and returns them normalized. Refused
// (validation error): unknown kinds, templates, block types or options, duplicate or invalid
// ids and slugs, a built-in tab twice, a custom tab without title, more tabs, sections or
// blocks than the limits. Missing built-in tabs are appended.
func ValidateProfileTabs(tabs data.ProfileTabs, limits ProfileTabLimits) (data.ProfileTabs, error) {
	if tabs == nil {
		return nil, profileTabsError("tabs is required")
	}
	if len(tabs) > limits.MaxTabs {
		return nil, profileTabsError(fmt.Sprintf("at most %d tabs", limits.MaxTabs))
	}
	ids := map[string]bool{}
	slugs := map[string]bool{}
	kinds := map[string]bool{}
	blockIDs := map[string]bool{}
	for i := range tabs {
		t := &tabs[i]
		if !profileIDPattern.MatchString(t.ID) || ids[t.ID] {
			return nil, profileTabsError(fmt.Sprintf("tab %d: the id must be unique, 1-32 lowercase letters, digits, - or _", i+1))
		}
		ids[t.ID] = true
		switch t.Kind {
		case data.ProfileTabCustom:
			t.Title = cleanText(t.Title, false)
			if n := utf8.RuneCountInString(t.Title); n == 0 || n > profileTabTitleMax {
				return nil, profileTabsError(fmt.Sprintf("tab %d: the title must be 1-%d characters", i+1, profileTabTitleMax))
			}
			if _, builtin := builtinTab(t.ID); builtin {
				return nil, profileTabsError(fmt.Sprintf("tab %q: this id is reserved", t.ID))
			}
			if t.Slug == "" {
				t.Slug = ProfileSlug(t.Title)
			}
			if t.Slug == "" {
				t.Slug = t.ID
			}
			if !profileSlugPattern.MatchString(t.Slug) || slugs[t.Slug] {
				return nil, profileTabsError(fmt.Sprintf("tab %q: the slug must be unique, lowercase letters, digits and -", t.Title))
			}
			slugs[t.Slug] = true
			if t.Icon != "" && (len(t.Icon) > 64 || !profileIconPattern.MatchString(t.Icon)) {
				return nil, profileTabsError(fmt.Sprintf("tab %q: invalid icon", t.Title))
			}
		default:
			if _, ok := builtinTab(t.Kind); !ok {
				return nil, profileTabsError(fmt.Sprintf("tab %d: unknown kind %q", i+1, t.Kind))
			}
			if kinds[t.Kind] {
				return nil, profileTabsError(fmt.Sprintf("the %s tab appears twice", t.Kind))
			}
			kinds[t.Kind] = true
			if t.ID != t.Kind {
				return nil, profileTabsError(fmt.Sprintf("the %s tab must have the id %q", t.Kind, t.Kind))
			}
		}
		if !tabHasSections(t.Kind) {
			if len(t.Sections) > 0 {
				return nil, profileTabsError(fmt.Sprintf("the %s tab has no sections", t.Kind))
			}
			continue
		}
		if err := validateSections(t, limits, blockIDs); err != nil {
			return nil, err
		}
	}
	return NormalizeProfileTabs(tabs, nil), nil
}

func validateSections(t *data.ProfileTab, limits ProfileTabLimits, blockIDs map[string]bool) error {
	name := t.Title
	if name == "" {
		name = t.Kind
	}
	if len(t.Sections) > limits.MaxSections {
		return profileTabsError(fmt.Sprintf("tab %q: at most %d sections", name, limits.MaxSections))
	}
	sectionIDs := map[string]bool{}
	blocks := 0
	for si := range t.Sections {
		sec := &t.Sections[si]
		if !profileIDPattern.MatchString(sec.ID) || sectionIDs[sec.ID] {
			return profileTabsError(fmt.Sprintf("tab %q, section %d: invalid or duplicate id", name, si+1))
		}
		sectionIDs[sec.ID] = true
		tpl, ok := sectionTemplate(sec.Template)
		if !ok {
			return profileTabsError(fmt.Sprintf("tab %q, section %d: unknown template %q", name, si+1, sec.Template))
		}
		if len(sec.Columns) != len(tpl.Widths) {
			return profileTabsError(fmt.Sprintf("tab %q, section %d: the %s template has %d columns", name, si+1, tpl.ID, len(tpl.Widths)))
		}
		for ci := range sec.Columns {
			for bi := range sec.Columns[ci] {
				b := &sec.Columns[ci][bi]
				blocks++
				if !profileIDPattern.MatchString(b.ID) || blockIDs[b.ID] {
					return profileTabsError(fmt.Sprintf("tab %q: block ids must be unique (%q)", name, b.ID))
				}
				blockIDs[b.ID] = true
				spec, ok := profileBlocks[b.Type]
				if !ok {
					return profileTabsError(fmt.Sprintf("unknown block type %q", b.Type))
				}
				cfg := make(map[string]any, len(b.Config))
				for key, raw := range b.Config {
					opt, ok := spec.option(key)
					if !ok {
						return profileTabsError(fmt.Sprintf("block %q has no option %q", b.Type, key))
					}
					v, err := opt.normalize(raw)
					if err != nil {
						return profileTabsError(fmt.Sprintf("block %q: %v", b.Type, err))
					}
					cfg[key] = v
				}
				b.Config = cfg
			}
		}
	}
	if blocks > limits.MaxBlocksPerTab {
		return profileTabsError(fmt.Sprintf("tab %q: at most %d blocks", name, limits.MaxBlocksPerTab))
	}
	return nil
}

// tabBlocks lists the blocks of a tab, section by section, column by column.
func tabBlocks(t data.ProfileTab) []data.ProfileBlockConfig {
	var out []data.ProfileBlockConfig
	for _, sec := range t.Sections {
		for _, col := range sec.Columns {
			out = append(out, col...)
		}
	}
	return out
}

// findTab returns the tab whose id or slug is key ("" = the profile tab).
func findTab(tabs data.ProfileTabs, key string) (data.ProfileTab, bool) {
	if key == "" {
		key = data.ProfileTabProfile
	}
	for _, t := range tabs {
		if t.ID == key || (t.Kind == data.ProfileTabCustom && t.Slug == key) {
			return t, true
		}
	}
	return data.ProfileTab{}, false
}
