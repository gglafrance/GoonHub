package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
)

// NavLayoutChangedEvent tells the other tabs and devices of a user that their header layout
// changed (personal, PublishToUser, no data: the client reads GET /settings/navigation).
const NavLayoutChangedEvent = "settings:nav_layout"

var (
	navFolderIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	navFolderIconPattern = regexp.MustCompile(`^heroicons:[a-z0-9]+(-[a-z0-9]+)*$`)
	navBarOrder          = []string{NavBarTop, NavBarNav}
	navZoneOrder         = []string{NavZoneLeft, NavZoneCenter, NavZoneRight}
)

// NavLayoutView is a user's header layout as they may see it.
type NavLayoutView struct {
	Layout data.NavLayout `json:"layout"`
	// Customized is false when the user has no stored layout (the default one is returned)
	Customized bool `json:"customized"`
}

// NavLayoutService stores the per-account layouts of the header bars (docs/NAVIGATION.md).
// A layout never shows an item the user lacks the permission for: the items are left out when
// the layout is read and when it is saved.
type NavLayoutService struct {
	repo     data.NavLayoutRepository
	eventBus *EventBus
	logger   *zap.Logger
}

func NewNavLayoutService(repo data.NavLayoutRepository, eventBus *EventBus, logger *zap.Logger) *NavLayoutService {
	return &NavLayoutService{repo: repo, eventBus: eventBus, logger: logger}
}

// Get returns the layout of the user: the stored one (repaired if the catalog changed since)
// or the default one, without the items the user may not see.
func (s *NavLayoutService) Get(ctx context.Context, access *UserAccess) (*NavLayoutView, error) {
	if access == nil {
		return nil, apperrors.NewUnauthorizedError("not authenticated")
	}
	stored, err := s.repo.Get(ctx, access.UserID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to read the header layout", err)
	}
	if stored == nil {
		layout, _ := normalizeNavLayout(DefaultNavLayout(), access.Has, false)
		return &NavLayoutView{Layout: layout, Customized: false}, nil
	}
	layout, _ := normalizeNavLayout(*stored, access.Has, false)
	return &NavLayoutView{Layout: layout, Customized: true}, nil
}

// Save validates and stores the layout of the user. Structural problems (unknown ids,
// duplicates, wrong bar, too many folders...) are refused; the items the user lacks the
// permission for are left out of the stored layout.
func (s *NavLayoutService) Save(ctx context.Context, access *UserAccess, layout data.NavLayout) (*NavLayoutView, error) {
	if access == nil {
		return nil, apperrors.NewUnauthorizedError("not authenticated")
	}
	clean, err := normalizeNavLayout(layout, access.Has, true)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, access.UserID, clean); err != nil {
		return nil, apperrors.NewInternalError("failed to save the header layout", err)
	}
	s.notify(access.UserID)
	return &NavLayoutView{Layout: clean, Customized: true}, nil
}

// Reset removes the stored layout: the user gets the default layout again.
func (s *NavLayoutService) Reset(ctx context.Context, access *UserAccess) (*NavLayoutView, error) {
	if access == nil {
		return nil, apperrors.NewUnauthorizedError("not authenticated")
	}
	if err := s.repo.Delete(ctx, access.UserID); err != nil {
		return nil, apperrors.NewInternalError("failed to reset the header layout", err)
	}
	s.notify(access.UserID)
	layout, _ := normalizeNavLayout(DefaultNavLayout(), access.Has, false)
	return &NavLayoutView{Layout: layout, Customized: false}, nil
}

func (s *NavLayoutService) notify(userID uint) {
	if s.eventBus == nil {
		return
	}
	s.eventBus.PublishToUser(userID, SceneEvent{Type: NavLayoutChangedEvent})
}

// navLayoutNormalizer walks a layout. In strict mode (a layout being saved) the first
// structural problem is returned as a validation error; otherwise (a stored layout read back,
// possibly from an older catalog) the faulty entries are dropped, and the frontend puts the
// dropped items back at their default place. In both modes the items the user lacks the
// permission for are left out silently.
type navLayoutNormalizer struct {
	has    func(string) bool
	strict bool
	seen   map[string]bool
	err    error
}

func (n *navLayoutNormalizer) fail(format string, args ...any) bool {
	if n.strict && n.err == nil {
		n.err = apperrors.NewValidationError(fmt.Sprintf(format, args...))
	}
	return false
}

// keepItem reports whether an item entry stays (bar: where it is placed).
func (n *navLayoutNormalizer) keepItem(id, bar string, inFolder bool) bool {
	spec, ok := navItemsByID[id]
	if !ok {
		return n.fail("unknown header item %q", id)
	}
	if n.seen[id] {
		return n.fail("header item %q is placed more than once", id)
	}
	if !spec.allowsBar(bar) {
		return n.fail("header item %q cannot go in the %s bar", id, bar)
	}
	if inFolder && !spec.Folderable {
		return n.fail("header item %q cannot go in a folder", id)
	}
	n.seen[id] = true
	return spec.Permission == "" || n.has(spec.Permission)
}

// cleanFolder checks the id, name and icon of a folder; false drops it (lenient mode).
func (n *navLayoutNormalizer) cleanFolder(f *data.NavFolder) bool {
	if !navFolderIDPattern.MatchString(f.ID) {
		return n.fail("invalid folder id %q", f.ID)
	}
	name := strings.TrimSpace(f.Name)
	invalidName := name == "" || utf8.RuneCountInString(name) > MaxNavFolderNameLength ||
		strings.IndexFunc(name, unicode.IsControl) >= 0
	if invalidName {
		if n.strict {
			return n.fail("folder names must have 1 to %d characters", MaxNavFolderNameLength)
		}
		name = "Folder"
	}
	f.Name = name
	if f.Icon != "" && (len(f.Icon) > MaxNavFolderIconLength || !navFolderIconPattern.MatchString(f.Icon)) {
		if n.strict {
			return n.fail("invalid folder icon %q", f.Icon)
		}
		f.Icon = ""
	}
	return true
}

func normalizeNavLayout(in data.NavLayout, has func(string) bool, strict bool) (data.NavLayout, error) {
	n := &navLayoutNormalizer{has: has, strict: strict, seen: map[string]bool{}}
	if in.Version != NavLayoutVersion {
		n.fail("unsupported header layout version %d", in.Version)
	}
	out := data.NavLayout{Version: NavLayoutVersion}
	out.Normalize()

	// Folders: valid and unique ids, at most MaxNavFolders
	if len(in.Folders) > MaxNavFolders {
		n.fail("at most %d folders", MaxNavFolders)
	}
	folders := map[string]*data.NavFolder{}
	var folderOrder []string
	for _, f := range in.Folders {
		if len(folderOrder) >= MaxNavFolders {
			break
		}
		f := f
		if !n.cleanFolder(&f) {
			continue
		}
		if folders[f.ID] != nil {
			n.fail("folder %q is defined more than once", f.ID)
			continue
		}
		folders[f.ID] = &f
		folderOrder = append(folderOrder, f.ID)
	}

	// Zones: items and folder references, each once
	folderBar := map[string]string{}
	for _, bar := range navBarOrder {
		for _, zone := range navZoneOrder {
			src := *navZone(&in, bar, zone)
			dst := navZone(&out, bar, zone)
			for _, entry := range src {
				if fid, ok := strings.CutPrefix(entry, NavFolderRefPrefix); ok {
					if folders[fid] == nil {
						n.fail("unknown folder %q", fid)
						continue
					}
					if _, placed := folderBar[fid]; placed {
						n.fail("folder %q is placed more than once", fid)
						continue
					}
					folderBar[fid] = bar
					*dst = append(*dst, entry)
					continue
				}
				if n.keepItem(entry, bar, false) {
					*dst = append(*dst, entry)
				}
			}
		}
	}

	// Folder items, in the bar of their folder; an unplaced folder goes to the end of the nav row
	for _, fid := range folderOrder {
		f := folders[fid]
		bar, placed := folderBar[fid]
		if !placed {
			n.fail("folder %q is not placed in a bar", fid)
			bar = NavBarNav
			out.Bars.Nav.Left = append(out.Bars.Nav.Left, NavFolderRefPrefix+fid)
		}
		items := []string{}
		for _, id := range f.Items {
			if n.keepItem(id, bar, true) {
				items = append(items, id)
			}
		}
		out.Folders = append(out.Folders, data.NavFolder{ID: f.ID, Name: f.Name, Icon: f.Icon, Items: items})
	}

	// Hidden items: known, not placed, not required
	for _, id := range in.Hidden {
		spec, ok := navItemsByID[id]
		switch {
		case !ok:
			n.fail("unknown header item %q", id)
		case n.seen[id]:
			n.fail("header item %q is both placed and hidden", id)
		case spec.Required:
			n.fail("header item %q cannot be hidden", id)
		default:
			n.seen[id] = true
			if spec.Permission == "" || has(spec.Permission) {
				out.Hidden = append(out.Hidden, id)
			}
		}
	}

	if n.err != nil {
		return data.NavLayout{}, n.err
	}
	return out, nil
}
