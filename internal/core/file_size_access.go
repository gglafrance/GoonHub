package core

import "goonhub/internal/data"

// Stored size filters and sorts (saved searches, settings, homepage sections) and users
// without files:view_size: they cannot save them (the API refuses), and those stored before
// migration 094 are ignored when executed (default sort, no size bound) instead of failing.

// DefaultSceneSort is the scene sort used when a stored sort cannot be applied.
const DefaultSceneSort = "created_at_desc"

// IsSizeSort reports whether a scene sort orders by file size.
func IsSizeSort(sort string) bool {
	return sort == "size_asc" || sort == "size_desc"
}

// SavedSearchUsesSize reports whether saved search filters use a size bound or a size sort.
func SavedSearchUsesSize(f data.Filters) bool {
	return (f.MinSize != nil && *f.MinSize > 0) || (f.MaxSize != nil && *f.MaxSize > 0) || IsSizeSort(f.Sort)
}

// WithoutSizeFilters returns the filters without size bounds nor size sort.
func WithoutSizeFilters(f data.Filters) data.Filters {
	f.MinSize = nil
	f.MaxSize = nil
	if IsSizeSort(f.Sort) {
		f.Sort = ""
	}
	return f
}

// HomepageUsesSizeSort reports whether a homepage section is sorted by size.
func HomepageUsesSizeSort(h *data.HomepageConfig) bool {
	if h == nil {
		return false
	}
	for _, s := range h.Sections {
		if IsSizeSort(s.Sort) {
			return true
		}
	}
	for _, t := range h.Tabs {
		for _, s := range t.Sections {
			if IsSizeSort(s.Sort) {
				return true
			}
		}
	}
	return false
}

// SettingsUseSizeSort reports whether settings use a size sort (default sort, homepage
// sections, actor/studio scene sorts). nil parts are not checked.
func SettingsUseSizeSort(defaultSort *string, homepage *data.HomepageConfig, prefs *data.SortPreferences) bool {
	if defaultSort != nil && IsSizeSort(*defaultSort) {
		return true
	}
	if prefs != nil && (IsSizeSort(prefs.ActorScenes) || IsSizeSort(prefs.StudioScenes)) {
		return true
	}
	return HomepageUsesSizeSort(homepage)
}

// withoutSizeSections copies sections, replacing size sorts with the section default ("").
func withoutSizeSections(sections []data.HomepageSection) []data.HomepageSection {
	if sections == nil {
		return nil
	}
	out := make([]data.HomepageSection, len(sections))
	for i, s := range sections {
		if IsSizeSort(s.Sort) {
			s.Sort = ""
		}
		out[i] = s
	}
	return out
}

// WithoutSizeSorts returns a copy of the settings where every size sort is replaced with
// the default sort (the settings themselves are not modified).
func WithoutSizeSorts(s *data.UserSettings) *data.UserSettings {
	if s == nil {
		return nil
	}
	cp := *s
	if IsSizeSort(cp.DefaultSortOrder) {
		cp.DefaultSortOrder = DefaultSceneSort
	}
	if IsSizeSort(cp.SortPreferences.ActorScenes) {
		cp.SortPreferences.ActorScenes = ""
	}
	if IsSizeSort(cp.SortPreferences.StudioScenes) {
		cp.SortPreferences.StudioScenes = ""
	}
	cp.HomepageConfig.Sections = withoutSizeSections(s.HomepageConfig.Sections)
	if s.HomepageConfig.Tabs != nil {
		cp.HomepageConfig.Tabs = make([]data.HomepageTab, len(s.HomepageConfig.Tabs))
		for i, t := range s.HomepageConfig.Tabs {
			t.Sections = withoutSizeSections(t.Sections)
			cp.HomepageConfig.Tabs[i] = t
		}
	}
	return &cp
}
