package core

// Permission names checked by the code. The catalog below is the reference (docs/RBAC.md).
const (
	PermScenesView        = "scenes:view"
	PermScenesUpload      = "scenes:upload"
	PermScenesEdit        = "scenes:edit"
	PermScenesTrash       = "scenes:trash"
	PermScenesDelete      = "scenes:delete"
	PermScenesReprocess   = "scenes:reprocess"
	PermScenesShare       = "scenes:share"
	PermMarkersCreate     = "markers:create"
	PermTagsManage        = "tags:manage"
	PermActorsManage      = "actors:manage"
	PermActorsMerge       = "actors:merge"
	PermStudiosManage     = "studios:manage"
	PermStudiosMerge      = "studios:merge"
	PermPlaylistsCreate   = "playlists:create"
	PermPlaylistsEdit     = "playlists:edit"
	PermPlaylistsDelete   = "playlists:delete"
	PermPlaylistsPublic   = "playlists:view_public"
	PermFilesViewSize     = "files:view_size"
	PermFilesViewPaths    = "files:view_paths"
	PermFilesViewTech     = "files:view_technical"
	PermExplorerAccess    = "explorer:access"
	PermJobsViewStatus    = "jobs:view_status"
	PermJobsManage        = "jobs:manage"
	PermUsersManage       = "users:manage"
	PermRolesManage       = "roles:manage"
	PermSettingsManage    = "settings:manage"
	PermStorageManage     = "storage:manage"
	PermDuplicatesManage  = "duplicates:manage"
	PermTrashManage       = "trash:manage"
	PermLibraryHealth     = "library:health"
	PermLibraryInsights   = "library:insights"
	PermPornDBUse         = "porndb:use"
	PermImportUse         = "import:use"
	PermStatsViewStreams  = "stats:view_streams"
	permUsersCreateLegacy = "users:create"
	permUsersDeleteLegacy = "users:delete"
)

// Permission groups (shown by the UI).
const (
	PermGroupScenes     = "Scenes"
	PermGroupMarkers    = "Markers"
	PermGroupLibrary    = "Library"
	PermGroupPlaylists  = "Playlists"
	PermGroupFileInfo   = "File info"
	PermGroupProcessing = "Processing"
	PermGroupAdmin      = "Admin"
)

// CatalogPermission is a permission of the catalog, with the default grants applied the
// first time it is inserted.
type CatalogPermission struct {
	Name        string
	Group       string
	Description string
	// GrantRoles are the roles (by name, those that exist) given the permission when it is
	// first inserted.
	GrantRoles []string
	// GrantFrom gives the permission, when it is first inserted, to every role that has this
	// other permission allowed (its own setting) at that time.
	GrantFrom string
}

// PermissionCatalog lists the permissions, in display order. "existing" permissions (already
// in the database) need no default grant: they are only kept up to date (group,
// description). The superuser role (admin) needs no grant.
var PermissionCatalog = []CatalogPermission{
	{Name: PermScenesView, Group: PermGroupScenes, Description: "Browse and watch scenes"},
	{Name: PermScenesUpload, Group: PermGroupScenes, Description: "Upload scenes"},
	{Name: PermScenesEdit, Group: PermGroupScenes, Description: "Edit scene details, tags, actors, studio and thumbnail", GrantRoles: []string{"moderator"}},
	{Name: PermScenesTrash, Group: PermGroupScenes, Description: "Move scenes to the trash"},
	{Name: PermScenesDelete, Group: PermGroupScenes, Description: "Delete scenes and their files (explorer bulk delete)"},
	{Name: PermScenesReprocess, Group: PermGroupScenes, Description: "Reprocess a scene"},
	{Name: PermScenesShare, Group: PermGroupScenes, Description: "Create and delete share links", GrantFrom: PermScenesView},
	{Name: PermMarkersCreate, Group: PermGroupMarkers, Description: "Create, edit and delete one's own markers and marker tags", GrantFrom: PermScenesView},
	{Name: PermTagsManage, Group: PermGroupLibrary, Description: "Create, rename, recolor, merge and delete tags", GrantRoles: []string{"moderator"}},
	{Name: PermActorsManage, Group: PermGroupLibrary, Description: "Create, edit and delete actors and their images"},
	{Name: PermActorsMerge, Group: PermGroupLibrary, Description: "Merge actors"},
	{Name: PermStudiosManage, Group: PermGroupLibrary, Description: "Create, edit and delete studios and their logos"},
	{Name: PermStudiosMerge, Group: PermGroupLibrary, Description: "Merge studios"},
	{Name: PermPlaylistsCreate, Group: PermGroupPlaylists, Description: "Create playlists"},
	{Name: PermPlaylistsEdit, Group: PermGroupPlaylists, Description: "Edit one's own playlists"},
	{Name: PermPlaylistsDelete, Group: PermGroupPlaylists, Description: "Delete one's own playlists"},
	{Name: PermPlaylistsPublic, Group: PermGroupPlaylists, Description: "View and like public playlists of other users"},
	{Name: PermFilesViewSize, Group: PermGroupFileInfo, Description: "File sizes, size filters and sorts, selection and library totals, biggest file", GrantRoles: []string{"moderator"}},
	{Name: PermFilesViewPaths, Group: PermGroupFileInfo, Description: "Stored paths, original file names, storage paths, copies of a scene", GrantRoles: []string{"moderator"}},
	{Name: PermFilesViewTech, Group: PermGroupFileInfo, Description: "Codecs, bit rate, frame rate, file hash, fingerprint, processing errors", GrantRoles: []string{"moderator"}},
	{Name: PermExplorerAccess, Group: PermGroupFileInfo, Description: "Folder explorer", GrantRoles: []string{"moderator"}},
	{Name: PermJobsViewStatus, Group: PermGroupProcessing, Description: "Job status in the header and its live events", GrantRoles: []string{"moderator"}},
	{Name: PermJobsManage, Group: PermGroupAdmin, Description: "Jobs, pools, processing, triggers, retry, DLQ, workers, transcode cache, scene phases"},
	{Name: PermUsersManage, Group: PermGroupAdmin, Description: "Users, their role, overrides, password, marker sharing"},
	{Name: PermRolesManage, Group: PermGroupAdmin, Description: "Roles, parents and role permissions"},
	{Name: PermSettingsManage, Group: PermGroupAdmin, Description: "App settings, search config and reindex"},
	{Name: PermStorageManage, Group: PermGroupAdmin, Description: "Storage paths and library scans"},
	{Name: PermDuplicatesManage, Group: PermGroupAdmin, Description: "Duplicates, copies, marker deduplication"},
	{Name: PermTrashManage, Group: PermGroupAdmin, Description: "Trash page: restore and permanent deletion"},
	{Name: PermLibraryHealth, Group: PermGroupAdmin, Description: "Library health"},
	{Name: PermLibraryInsights, Group: PermGroupAdmin, Description: "Curation and processing homepage widgets, trashed scenes in Similar"},
	{Name: PermPornDBUse, Group: PermGroupAdmin, Description: "PornDB search and applying scene metadata"},
	{Name: PermImportUse, Group: PermGroupAdmin, Description: "Import API"},
	{Name: PermStatsViewStreams, Group: PermGroupAdmin, Description: "Stream statistics"},
}

// DeprecatedPermissions are kept in the table but not shown nor checked anymore (folded
// into users:manage).
var DeprecatedPermissions = map[string]bool{
	permUsersCreateLegacy: true,
	permUsersDeleteLegacy: true,
}

// catalogIndex maps a catalog permission name to its position in the catalog.
func catalogIndex() map[string]int {
	idx := make(map[string]int, len(PermissionCatalog))
	for i, p := range PermissionCatalog {
		idx[p.Name] = i
	}
	return idx
}
