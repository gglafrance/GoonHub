package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"gorm.io/gorm"
)

// Custom lists of a profile (docs/PROFILES.md, "Custom lists"): search criteria (the saved
// search filters and the search sorts) run with the OWNER's interactions, shown by custom_list
// blocks and browsable on the profile. A criterion on the owner's interactions reveals owner
// data, so each one depends on a privacy item; a viewer gets a list only when they pass the
// profile item, the list's own audience and every item its criteria depend on, and have
// scenes:view (and files:view_size when the criteria use sizes). Scenes are redacted for the
// viewer and the text query skips paths without files:view_paths.

// ProfileSceneSearcher runs a scene search (*SearchService).
type ProfileSceneSearcher interface {
	Search(params data.SceneSearchParams) (*SearchResult, error)
}

// ProfileTagResolver resolves tag names (data.TagRepository).
type ProfileTagResolver interface {
	GetIDsByNames(names []string) ([]uint, error)
}

// WithCustomLists gives the service what the custom lists need: the search, the tags and the
// saved searches (a list may follow one).
func (s *ProfileService) WithCustomLists(searcher ProfileSceneSearcher, tags ProfileTagResolver, saved data.SavedSearchRepository) *ProfileService {
	s.searcher = searcher
	s.tags = tags
	s.savedSearches = saved
	return s
}

const (
	customListTitleMax  = 80
	customListQueryMax  = 200
	customListValuesMax = 50
)

// profileListSorts are the sorts a custom list may use (the search sorts).
var profileListSorts = map[string]bool{
	"": true, "relevance": true, "random": true, "title_asc": true, "title_desc": true,
	"duration_asc": true, "duration_desc": true, "size_asc": true, "size_desc": true,
	"created_at_asc": true, "created_at_desc": true, "view_count_desc": true, "view_count_asc": true,
	sortJizzCountDesc: true, sortJizzCountAsc: true, sortLastJizzDesc: true, sortLastJizzAsc: true,
	SortMyRatingDesc: true, SortMyRatingAsc: true, SortLikedAtDesc: true, SortMyViewsDesc: true, SortLastWatchedDesc: true,
}

// sortPrivacyItem is the privacy item a user-specific sort reveals.
func sortPrivacyItem(sort string) string {
	switch {
	case isJizzSortOrder(sort):
		return PrivacyJizz
	case sort == SortMyRatingDesc || sort == SortMyRatingAsc:
		return PrivacyRatings
	case sort == SortLikedAtDesc:
		return PrivacyLiked
	case sort == SortMyViewsDesc || sort == SortLastWatchedDesc:
		return PrivacyActivity
	}
	return ""
}

// ProfileListDependencies returns the privacy items the criteria depend on (catalog order) and
// whether they use file sizes:
//   - liked, like date sort: liked
//   - rating range, rated or unrated, rating sorts: ratings
//   - O range, O sorts: jizz
//   - watched or unwatched, view and last watch sorts of the user: activity
//   - with or without markers, marker labels: recent_markers
func ProfileListDependencies(f data.Filters) ([]string, bool) {
	set := map[string]bool{}
	if f.Liked != nil && *f.Liked {
		set[PrivacyLiked] = true
	}
	if (f.MinRating != nil && *f.MinRating > 0) || (f.MaxRating != nil && *f.MaxRating > 0) || f.Rated != nil {
		set[PrivacyRatings] = true
	}
	if (f.MinJizzCount != nil && *f.MinJizzCount > 0) || (f.MaxJizzCount != nil && *f.MaxJizzCount > 0) {
		set[PrivacyJizz] = true
	}
	if f.Watched != nil {
		set[PrivacyActivity] = true
	}
	if f.HasMarkers != nil || len(f.MarkerLabels) > 0 {
		set[PrivacyRecentMarkers] = true
	}
	if item := sortPrivacyItem(f.Sort); item != "" {
		set[item] = true
	}
	out := []string{}
	for _, item := range PrivacyCatalog {
		if set[item.Key] {
			out = append(out, item.Key)
		}
	}
	return out, SavedSearchUsesSize(f)
}

func customListError(field, msg string) error {
	return newProfileValidationError(field, msg)
}

// validateListFilters checks the criteria of a list (the owner needs files:view_size for size
// criteria, like a saved search) and returns them cleaned.
func validateListFilters(f data.Filters, owner ProfileViewer) (data.Filters, error) {
	f.Query = strings.TrimSpace(f.Query)
	if utf8.RuneCountInString(f.Query) > customListQueryMax {
		return f, customListError("filters.query", fmt.Sprintf("at most %d characters", customListQueryMax))
	}
	switch f.MatchType {
	case "", "broad", "strict", "frequency":
	default:
		return f, customListError("filters.match_type", "unknown match type")
	}
	for name, values := range map[string][]string{
		"selected_tags": f.SelectedTags, "selected_actors": f.SelectedActors,
		"source_sites": f.SourceSites, "selected_marker_labels": f.MarkerLabels,
	} {
		if len(values) > customListValuesMax {
			return f, customListError("filters."+name, fmt.Sprintf("at most %d values", customListValuesMax))
		}
		for _, v := range values {
			if v == "" || utf8.RuneCountInString(v) > 255 {
				return f, customListError("filters."+name, "invalid value")
			}
		}
	}
	if utf8.RuneCountInString(f.Studio) > 255 {
		return f, customListError("filters.studio", "too long")
	}
	if _, ok := resolutionHeights[f.Resolution]; f.Resolution != "" && !ok {
		return f, customListError("filters.resolution", "unknown resolution")
	}
	for name, v := range map[string]*int{"min_duration": f.MinDuration, "max_duration": f.MaxDuration, "min_size": f.MinSize,
		"max_size": f.MaxSize, "min_jizz_count": f.MinJizzCount, "max_jizz_count": f.MaxJizzCount} {
		if v != nil && (*v < 0 || *v > 1_000_000_000) {
			return f, customListError("filters."+name, "out of range")
		}
	}
	for name, v := range map[string]*float64{"min_rating": f.MinRating, "max_rating": f.MaxRating} {
		if v != nil && (*v < 0 || *v > ProfileRatingMax) {
			return f, customListError("filters."+name, "a rating is between 0 and 5")
		}
	}
	for name, v := range map[string]string{"min_date": f.MinDate, "max_date": f.MaxDate} {
		if v != "" {
			if _, err := time.Parse("2006-01-02", v); err != nil {
				return f, customListError("filters."+name, "a date is YYYY-MM-DD")
			}
		}
	}
	if !profileListSorts[f.Sort] {
		return f, customListError("filters.sort", fmt.Sprintf("unknown sort %q", f.Sort))
	}
	if SavedSearchUsesSize(f) && !owner.Has(PermFilesViewSize) {
		return f, apperrors.NewForbiddenError("size criteria need the files:view_size permission")
	}
	return f, nil
}

// resolutionHeights maps the resolution filter to heights (as the scene search does).
var resolutionHeights = map[string][2]int{
	"4k": {2160, 0}, "1440p": {1440, 2159}, "1080p": {1080, 1439}, "720p": {720, 1079}, "480p": {480, 719}, "360p": {0, 479},
}

// ResolvedProfileList is a list with its effective criteria (those of the saved search it
// follows, while that exists) and what they depend on.
type ResolvedProfileList struct {
	List      data.ProfileList
	Filters   data.Filters
	Deps      []string
	NeedsSize bool
	// FollowsSavedSearch: the criteria come from the live saved search
	FollowsSavedSearch bool
}

// resolveList reads the effective criteria of a list.
func (s *ProfileService) resolveList(l data.ProfileList) ResolvedProfileList {
	out := ResolvedProfileList{List: l, Filters: l.Filters}
	if l.SavedSearchUUID != nil && s.savedSearches != nil {
		if ss, err := s.savedSearches.GetByUUID(*l.SavedSearchUUID); err == nil && ss.UserID == l.UserID {
			out.Filters = ss.Filters
			out.FollowsSavedSearch = true
		}
	}
	out.Deps, out.NeedsSize = ProfileListDependencies(out.Filters)
	return out
}

// listVisibleTo tells whether a viewer gets a list: scenes:view (and files:view_size for size
// criteria), the list's audience and every item its criteria depend on (visible: the
// visibility of the owner's items for the viewer, the profile item already checked).
func listVisibleTo(viewer ProfileViewer, rel Relationship, visible map[string]bool, rl ResolvedProfileList) bool {
	if !viewer.Has(PermScenesView) || (rl.NeedsSize && !viewer.Has(PermFilesViewSize)) {
		return false
	}
	if !rel.Passes(Audience(rl.List.Audience)) {
		return false
	}
	for _, item := range rl.Deps {
		if !visible[item] {
			return false
		}
	}
	return true
}

// searchParams maps criteria to the search, run with the owner's interactions and the viewer's
// file permissions. ok is false when the criteria name tags that do not exist (nothing can
// match: dropping them would widen the list).
func (s *ProfileService) searchParams(f data.Filters, ownerID uint, viewer ProfileViewer, page, limit int) (data.SceneSearchParams, bool, error) {
	if !viewer.Has(PermFilesViewSize) {
		f = WithoutSizeFilters(f)
	}
	strategy := "last"
	switch f.MatchType {
	case "strict":
		strategy = "all"
	case "frequency":
		strategy = "frequency"
	}
	sortOrder := f.Sort
	if sortOrder == "" {
		sortOrder = "created_at_desc"
	}
	p := data.SceneSearchParams{
		Page: page, Limit: limit, Query: f.Query, Studio: f.Studio, Sort: sortOrder, UserID: ownerID,
		MatchingStrategy: strategy, HideFilePaths: !viewer.Has(PermFilesViewPaths),
		HasActors: f.HasActors, HasTags: f.HasTags, HasStudio: f.HasStudio, HasSource: f.HasSource,
		SourceSites: NormalizeSourceSites(f.SourceSites), Watched: f.Watched, Rated: f.Rated, HasMarkers: f.HasMarkers,
		Liked: f.Liked, MarkerLabels: f.MarkerLabels, Actors: f.SelectedActors,
	}
	deref := func(v *int) int {
		if v == nil {
			return 0
		}
		return *v
	}
	p.MinDuration, p.MaxDuration = deref(f.MinDuration), deref(f.MaxDuration)
	p.MinSizeMB, p.MaxSizeMB = deref(f.MinSize), deref(f.MaxSize)
	p.MinJizzCount, p.MaxJizzCount = deref(f.MinJizzCount), deref(f.MaxJizzCount)
	if f.MinRating != nil {
		p.MinRating = *f.MinRating
	}
	if f.MaxRating != nil {
		p.MaxRating = *f.MaxRating
	}
	if t, err := time.Parse("2006-01-02", f.MinDate); err == nil {
		p.MinDate = &t
	}
	if t, err := time.Parse("2006-01-02", f.MaxDate); err == nil {
		end := t.Add(24*time.Hour - time.Second)
		p.MaxDate = &end
	}
	if h, ok := resolutionHeights[f.Resolution]; ok {
		p.MinHeight, p.MaxHeight = h[0], h[1]
	}
	if len(f.SelectedTags) > 0 {
		if s.tags == nil {
			return p, false, nil
		}
		ids, err := s.tags.GetIDsByNames(f.SelectedTags)
		if err != nil {
			return p, false, fmt.Errorf("failed to resolve the tags: %w", err)
		}
		if len(ids) == 0 {
			return p, false, nil
		}
		p.TagIDs = ids
	}
	return p, true, nil
}

// runList runs the criteria of a list of ownerID for a viewer (already allowed to see it).
func (s *ProfileService) runList(_ context.Context, ownerID uint, viewer ProfileViewer, f data.Filters, page, limit int) ([]data.Scene, int64, error) {
	if s.searcher == nil {
		return nil, 0, fmt.Errorf("the search is not available")
	}
	params, ok, err := s.searchParams(f, ownerID, viewer, page, limit)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return []data.Scene{}, 0, nil
	}
	res, err := s.searcher.Search(params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to run the list: %w", err)
	}
	scenes := make([]data.Scene, 0, len(res.Scenes))
	for _, sc := range res.Scenes {
		if sc.TrashedAt == nil {
			scenes = append(scenes, sc)
		}
	}
	return scenes, res.Total, nil
}

// --- The owner's lists ---

// CustomListInput creates or replaces a list. SavedSearchUUID (optional) makes the list
// follow a saved search of the owner; its filters are copied into Filters too.
type CustomListInput struct {
	Title           string
	Filters         data.Filters
	SavedSearchUUID string
	Audience        string
}

// CustomListDependency is a privacy item a list depends on, with the owner's audience.
type CustomListDependency struct {
	Key      string
	Label    string
	Audience Audience
}

// CustomListInfo is a list as its owner edits it.
type CustomListInfo struct {
	ResolvedProfileList
	Dependencies []CustomListDependency
}

func (s *ProfileService) listInfos(ctx context.Context, userID uint, lists []data.ProfileList) ([]CustomListInfo, error) {
	settings, err := s.privacy.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	audiences := map[string]PrivacyItemSetting{}
	for _, it := range settings.Items {
		audiences[it.Item.Key] = it
	}
	out := make([]CustomListInfo, len(lists))
	for i, l := range lists {
		rl := s.resolveList(l)
		info := CustomListInfo{ResolvedProfileList: rl, Dependencies: []CustomListDependency{}}
		for _, key := range rl.Deps {
			it := audiences[key]
			info.Dependencies = append(info.Dependencies, CustomListDependency{Key: key, Label: it.Item.Label, Audience: it.Audience})
		}
		out[i] = info
	}
	return out, nil
}

// MyLists returns the owner's lists with their dependencies.
func (s *ProfileService) MyLists(ctx context.Context, userID uint) ([]CustomListInfo, error) {
	lists, err := s.repo.Lists(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the lists: %w", err)
	}
	return s.listInfos(ctx, userID, lists)
}

func (s *ProfileService) checkListInput(owner ProfileViewer, in *CustomListInput, list *data.ProfileList) error {
	title := cleanText(in.Title, false)
	if n := utf8.RuneCountInString(title); n == 0 || n > customListTitleMax {
		return customListError("title", fmt.Sprintf("the title must be 1-%d characters", customListTitleMax))
	}
	audience := Audience(in.Audience)
	if in.Audience == "" {
		audience = AudiencePublic
	}
	if !audience.Valid() {
		return customListError("audience", "unknown audience")
	}
	list.SavedSearchUUID = nil
	filters := in.Filters
	if in.SavedSearchUUID != "" {
		if s.savedSearches == nil {
			return customListError("saved_search_uuid", "saved searches are not available")
		}
		ss, err := s.savedSearches.GetByUUID(in.SavedSearchUUID)
		if err != nil || ss.UserID != owner.UserID {
			return customListError("saved_search_uuid", "unknown saved search")
		}
		filters = ss.Filters
		u := ss.UUID.String()
		list.SavedSearchUUID = &u
	}
	f, err := validateListFilters(filters, owner)
	if err != nil {
		return err
	}
	list.Title, list.Filters, list.Audience = title, f, string(audience)
	return nil
}

// CreateList adds a list (at most profiles.max_lists per user).
func (s *ProfileService) CreateList(ctx context.Context, owner ProfileViewer, in CustomListInput) (*CustomListInfo, error) {
	n, err := s.repo.CountLists(ctx, owner.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to count the lists: %w", err)
	}
	if int(n) >= s.cfg.MaxLists {
		return nil, customListError("lists", fmt.Sprintf("at most %d lists", s.cfg.MaxLists))
	}
	list := data.ProfileList{UserID: owner.UserID}
	if err := s.checkListInput(owner, &in, &list); err != nil {
		return nil, err
	}
	if err := s.repo.CreateList(ctx, &list); err != nil {
		return nil, fmt.Errorf("failed to create the list: %w", err)
	}
	infos, err := s.listInfos(ctx, owner.UserID, []data.ProfileList{list})
	if err != nil {
		return nil, err
	}
	return &infos[0], nil
}

// UpdateList replaces a list of the owner.
func (s *ProfileService) UpdateList(ctx context.Context, owner ProfileViewer, id uint, in CustomListInput) (*CustomListInfo, error) {
	list, err := s.ownList(ctx, owner.UserID, id)
	if err != nil {
		return nil, err
	}
	if err := s.checkListInput(owner, &in, list); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateList(ctx, list); err != nil {
		return nil, fmt.Errorf("failed to save the list: %w", err)
	}
	infos, err := s.listInfos(ctx, owner.UserID, []data.ProfileList{*list})
	if err != nil {
		return nil, err
	}
	return &infos[0], nil
}

// DeleteList deletes a list of the owner (its blocks then show nothing and are dropped from
// the tabs at the next save).
func (s *ProfileService) DeleteList(ctx context.Context, userID, id uint) error {
	ok, err := s.repo.DeleteList(ctx, userID, id)
	if err != nil {
		return fmt.Errorf("failed to delete the list: %w", err)
	}
	if !ok {
		return apperrors.NewNotFoundError("list", id)
	}
	return nil
}

func (s *ProfileService) ownList(ctx context.Context, userID, id uint) (*data.ProfileList, error) {
	list, err := s.repo.GetList(ctx, userID, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.NewNotFoundError("list", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read the list: %w", err)
	}
	return list, nil
}

// --- Browse ---

// CustomListPage is a page of a list as a viewer browses it.
type CustomListPage struct {
	List  data.ProfileList
	Items []ProfileSceneItem
	Total int64
	Page  int
	Limit int
}

// BrowseList returns a page of a list of username, if the viewer may see it (profile item,
// the list's audience and dependencies). ErrPrivacyRestricted when hidden, not found when the
// list does not exist.
func (s *ProfileService) BrowseList(ctx context.Context, viewer ProfileViewer, username string, id uint, page, limit int) (*CustomListPage, error) {
	if !viewer.Has(PermScenesView) {
		return nil, apperrors.NewForbiddenError("insufficient permissions")
	}
	owner, err := s.social.UserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	list, err := s.ownList(ctx, owner.ID, id)
	if err != nil {
		return nil, err
	}
	rl := s.resolveList(*list)
	visible, err := s.privacy.Visibility(ctx, viewer.UserID, owner.ID, append([]string{PrivacyProfile}, rl.Deps...))
	if err != nil {
		return nil, err
	}
	rel, err := s.social.Relationship(ctx, viewer.UserID, owner.ID)
	if err != nil {
		return nil, err
	}
	if !visible[PrivacyProfile] || !listVisibleTo(viewer, rel, visible, rl) {
		return nil, apperrors.ErrPrivacyRestricted()
	}
	scenes, total, err := s.runList(ctx, owner.ID, viewer, rl.Filters, page, limit)
	if err != nil {
		return nil, err
	}
	items := make([]ProfileSceneItem, len(scenes))
	for i, sc := range scenes {
		items[i] = ProfileSceneItem{Scene: sc}
	}
	return &CustomListPage{List: *list, Items: items, Total: total, Page: page, Limit: limit}, nil
}

// customListIDs returns the list ids of the custom_list blocks of tabs.
func customListIDs(tabs data.ProfileTabs) []uint {
	seen := map[uint]bool{}
	var ids []uint
	for _, t := range tabs {
		for _, b := range tabBlocks(t) {
			if b.Type != customListBlockType {
				continue
			}
			spec := profileBlocks[b.Type]
			if id := uint(spec.resolve(b.Config).Int("list_id")); id > 0 && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
