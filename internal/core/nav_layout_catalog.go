package core

import "goonhub/internal/data"

// Header bars, zones and limits of the per-account header layout (docs/NAVIGATION.md).
const (
	NavBarTop = "top"
	NavBarNav = "nav"

	NavZoneLeft   = "left"
	NavZoneCenter = "center"
	NavZoneRight  = "right"

	// NavFolderRefPrefix starts a zone entry that places a folder ("folder:<id>")
	NavFolderRefPrefix = "folder:"
	// NavLayoutVersion is the version of the layout format
	NavLayoutVersion = 1
	// DefaultNavFolderID is the folder of the default layout (the former "Manage" menu)
	DefaultNavFolderID = "manage"

	MaxNavFolders          = 12
	MaxNavFolderNameLength = 32
	MaxNavFolderIconLength = 64
)

// NavItemSpec is an item of the header registry. The frontend registry
// (web/app/utils/navRegistry.ts) has the same ids, permissions, bars and flags; a frontend test
// reads this file to keep them in sync.
type NavItemSpec struct {
	ID string
	// Permission needed to see the item ("" = every signed-in user)
	Permission string
	// Bars the item can be placed in
	Bars []string
	// Required items cannot be hidden (the logo, settings and logout)
	Required bool
	// Folderable items can be put in a folder (they have a menu row rendering)
	Folderable bool
	// Default place: bar and zone, or the default folder
	DefaultBar    string
	DefaultZone   string
	DefaultFolder string
}

var (
	navBarsTopOnly = []string{NavBarTop}
	navBarsBoth    = []string{NavBarTop, NavBarNav}
)

// navItemCatalog lists every header item in its default order (today's header).
var navItemCatalog = []NavItemSpec{
	// Top bar
	{ID: "logo", Bars: navBarsTopOnly, Required: true, DefaultBar: NavBarTop, DefaultZone: NavZoneLeft},
	{ID: "command_palette", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	{ID: "profile", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	{ID: "shortcuts", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	{ID: "safe_mode", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	{ID: "preview_sound", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	{ID: "job_status", Permission: PermJobsViewStatus, Bars: navBarsBoth, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	{ID: "presence", Bars: navBarsBoth, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	{ID: "messages", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	{ID: "friends", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	{ID: "settings", Bars: navBarsBoth, Required: true, Folderable: true, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	{ID: "logout", Bars: navBarsBoth, Required: true, Folderable: true, DefaultBar: NavBarTop, DefaultZone: NavZoneRight},
	// Nav row: the user pages, then the Manage folder
	{ID: "search", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarNav, DefaultZone: NavZoneLeft},
	{ID: "markers", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarNav, DefaultZone: NavZoneLeft},
	{ID: "playlists", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarNav, DefaultZone: NavZoneLeft},
	{ID: "together", Permission: PermTogetherUse, Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarNav, DefaultZone: NavZoneLeft},
	{ID: "history", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarNav, DefaultZone: NavZoneLeft},
	{ID: "actors", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarNav, DefaultZone: NavZoneLeft},
	{ID: "studios", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarNav, DefaultZone: NavZoneLeft},
	{ID: "triage", Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarNav, DefaultZone: NavZoneLeft, DefaultFolder: DefaultNavFolderID},
	{ID: "explorer", Permission: PermExplorerAccess, Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarNav, DefaultZone: NavZoneLeft, DefaultFolder: DefaultNavFolderID},
	{ID: "duplicates", Permission: PermDuplicatesManage, Bars: navBarsBoth, Folderable: true, DefaultBar: NavBarNav, DefaultZone: NavZoneLeft, DefaultFolder: DefaultNavFolderID},
}

// defaultNavFolders are the folders of the default layout, placed in the zone of their first
// item, right after the items before it.
var defaultNavFolders = []data.NavFolder{
	{ID: DefaultNavFolderID, Name: "Manage", Icon: "heroicons:wrench-screwdriver"},
}

var navItemsByID = func() map[string]NavItemSpec {
	m := make(map[string]NavItemSpec, len(navItemCatalog))
	for _, spec := range navItemCatalog {
		m[spec.ID] = spec
	}
	return m
}()

// NavItemCatalog returns the header items, in default order.
func NavItemCatalog() []NavItemSpec {
	out := make([]NavItemSpec, len(navItemCatalog))
	copy(out, navItemCatalog)
	return out
}

// allowsBar reports whether the item can be placed in the bar.
func (s NavItemSpec) allowsBar(bar string) bool {
	for _, b := range s.Bars {
		if b == bar {
			return true
		}
	}
	return false
}

// navZone returns the zone list of a layout.
func navZone(l *data.NavLayout, bar, zone string) *[]string {
	z := &l.Bars.Top
	if bar == NavBarNav {
		z = &l.Bars.Nav
	}
	switch zone {
	case NavZoneCenter:
		return &z.Center
	case NavZoneRight:
		return &z.Right
	default:
		return &z.Left
	}
}

// DefaultNavLayout is the layout of a user who never customized the header: today's header.
func DefaultNavLayout() data.NavLayout {
	l := data.NavLayout{Version: NavLayoutVersion}
	l.Normalize()
	folderIndex := map[string]int{}
	for _, f := range defaultNavFolders {
		folderIndex[f.ID] = len(l.Folders)
		l.Folders = append(l.Folders, data.NavFolder{ID: f.ID, Name: f.Name, Icon: f.Icon, Items: []string{}})
	}
	placed := map[string]bool{}
	for _, spec := range navItemCatalog {
		if i, ok := folderIndex[spec.DefaultFolder]; ok {
			if !placed[spec.DefaultFolder] {
				zone := navZone(&l, spec.DefaultBar, spec.DefaultZone)
				*zone = append(*zone, NavFolderRefPrefix+spec.DefaultFolder)
				placed[spec.DefaultFolder] = true
			}
			l.Folders[i].Items = append(l.Folders[i].Items, spec.ID)
			continue
		}
		zone := navZone(&l, spec.DefaultBar, spec.DefaultZone)
		*zone = append(*zone, spec.ID)
	}
	return l
}
