package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MaxTrashBulk is the most scenes a bulk trash action (restore, permanent delete, select all)
// takes at a time.
const MaxTrashBulk = 500

// TrashListParams is a page of the trash page: filter, sort and pagination.
type TrashListParams struct {
	Filter data.TrashFilter
	Sort   string // data.TrashSort*, default trashed_at
	Desc   bool
	Page   int
	Limit  int
}

// TrashedScene is a scene of the trash page.
type TrashedScene struct {
	ID            uint   `json:"id"`
	Title         string `json:"title"`
	ThumbnailPath string `json:"thumbnail_path"`
	StoredPath    string `json:"stored_path"`
	StoragePathID *uint  `json:"storage_path_id"`
	// StoragePathName and RelativePath split the file path when the scene has a storage path
	StoragePathName string    `json:"storage_path_name"`
	RelativePath    string    `json:"relative_path"`
	Size            int64     `json:"size"`
	Duration        int       `json:"duration"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	VideoCodec      string    `json:"video_codec"`
	TrashedAt       time.Time `json:"trashed_at"`
	// ExpiresAt is when the scene will be purged: the retention, or the scheduled deletion of a
	// resolved duplicate when sooner
	ExpiresAt           time.Time  `json:"expires_at"`
	ScheduledDeletionAt *time.Time `json:"scheduled_deletion_at,omitempty"`
	// Source is data.TrashSourceDuplicates (not kept when its duplicate group was resolved) or
	// data.TrashSourceOther
	Source           string `json:"source"`
	DuplicateGroupID *uint  `json:"duplicate_group_id,omitempty"`
	// KeptOnDisk: the file is under a read-only storage path, a permanent deletion keeps it
	KeptOnDisk bool `json:"kept_on_disk"`
	// FileMissing: the last completed missing-file check found the file missing
	FileMissing bool `json:"file_missing"`
}

// TrashList is a page of the trash with the totals of the filter and of the whole trash.
type TrashList struct {
	Items         []TrashedScene
	Filtered      data.TrashTotals
	All           data.TrashTotals
	RetentionDays int
	Page          int
	Limit         int
}

// TrashBulkFailure is a scene a bulk trash action could not handle.
type TrashBulkFailure struct {
	SceneID uint   `json:"scene_id"`
	Error   string `json:"error"`
}

// TrashBulkResult is the outcome of a bulk trash action, per scene.
type TrashBulkResult struct {
	Succeeded []uint             `json:"succeeded"`
	Failed    []TrashBulkFailure `json:"failed"`
	// Size sums the size of the scenes handled
	Size int64 `json:"size"`
}

func trashQuery(p TrashListParams, retentionDays int) data.TrashQuery {
	sort := p.Sort
	if !data.IsValidTrashSort(sort) {
		sort = data.TrashSortTrashedAt
	}
	return data.TrashQuery{
		Filter:        p.Filter,
		Sort:          sort,
		Desc:          p.Desc,
		RetentionDays: retentionDays,
		Page:          p.Page,
		Limit:         p.Limit,
	}
}

// ListTrash returns a page of the trashed scenes matching the filter, with the count and size
// of the filter and of the whole trash.
func (s *SceneService) ListTrash(p TrashListParams) (*TrashList, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 {
		p.Limit = 20
	}
	retentionDays := s.GetTrashRetentionDays()
	rows, filtered, err := s.Repo.ListTrash(trashQuery(p, retentionDays))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list trashed scenes", err)
	}
	all := filtered
	if p.Filter != (data.TrashFilter{}) {
		if all, err = s.Repo.TrashTotals(data.TrashFilter{}); err != nil {
			return nil, apperrors.NewInternalError("failed to count trashed scenes", err)
		}
	}

	readOnly, err := s.mediaGuard.ReadOnlyChecker()
	if err != nil {
		s.logger.Warn("Trash list: could not check read-only storage paths", zap.Error(err))
		readOnly = func(*uint, string) bool { return false }
	}
	items := make([]TrashedScene, len(rows))
	for i, r := range rows {
		items[i] = toTrashedScene(r, readOnly(r.StoragePathID, r.StoredPath))
	}
	return &TrashList{
		Items:         items,
		Filtered:      filtered,
		All:           all,
		RetentionDays: retentionDays,
		Page:          p.Page,
		Limit:         p.Limit,
	}, nil
}

func toTrashedScene(r data.TrashedSceneRow, keptOnDisk bool) TrashedScene {
	item := TrashedScene{
		ID:                  r.ID,
		Title:               r.Title,
		ThumbnailPath:       r.ThumbnailPath,
		StoredPath:          r.StoredPath,
		StoragePathID:       r.StoragePathID,
		RelativePath:        r.StoredPath,
		Size:                r.Size,
		Duration:            r.Duration,
		Width:               r.Width,
		Height:              r.Height,
		VideoCodec:          r.VideoCodec,
		TrashedAt:           r.TrashedAt.UTC(),
		ExpiresAt:           r.PurgeAt.UTC(),
		ScheduledDeletionAt: r.DeleteAfter,
		Source:              data.TrashSourceOther,
		DuplicateGroupID:    r.DuplicateGroupID,
		KeptOnDisk:          keptOnDisk,
		FileMissing:         r.FileMissing,
	}
	if r.DuplicateGroupID != nil {
		item.Source = data.TrashSourceDuplicates
	}
	if r.StoragePathName != nil {
		item.StoragePathName = *r.StoragePathName
	}
	if r.StoragePathRoot != nil {
		item.RelativePath = relativeToRoot(r.StoredPath, *r.StoragePathRoot)
	}
	return item
}

// relativeToRoot returns the file path relative to a storage path root, or the full path when
// the file is not under it.
func relativeToRoot(file, root string) string {
	root = strings.TrimRight(root, `/\`)
	if root == "" || !strings.HasPrefix(file, root) {
		return file
	}
	rest := file[len(root):]
	if rest == "" || (rest[0] != '/' && rest[0] != '\\') {
		return file
	}
	return strings.TrimLeft(rest, `/\`)
}

// TrashIDs returns the IDs and sizes of the trashed scenes matching the filter in the list's
// sort (for "select all"), at most limit, with the number of matching scenes.
func (s *SceneService) TrashIDs(p TrashListParams, limit int) ([]data.TrashIDSize, int64, error) {
	items, total, err := s.Repo.ListTrashIDs(trashQuery(p, s.GetTrashRetentionDays()), limit)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("failed to list trashed scene ids", err)
	}
	return items, total, nil
}

// TrashFacets returns the codecs and storage paths of the whole trash (filter options).
func (s *SceneService) TrashFacets() (data.TrashFacets, error) {
	facets, err := s.Repo.TrashFacets()
	if err != nil {
		return facets, apperrors.NewInternalError("failed to list trash facets", err)
	}
	return facets, nil
}

// RestoreScenesFromTrash restores each scene (RestoreSceneFromTrash), reporting each failure.
func (s *SceneService) RestoreScenesFromTrash(ids []uint) TrashBulkResult {
	result := TrashBulkResult{Succeeded: []uint{}, Failed: []TrashBulkFailure{}}
	for _, id := range uniqueIDs(ids) {
		scene, err := s.Repo.GetByIDIncludingTrashed(id)
		if err != nil {
			result.Failed = append(result.Failed, TrashBulkFailure{SceneID: id, Error: sceneLookupError(err)})
			continue
		}
		if err := s.RestoreSceneFromTrash(id); err != nil {
			result.Failed = append(result.Failed, TrashBulkFailure{SceneID: id, Error: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, id)
		result.Size += scene.Size
	}
	return result
}

// DeleteTrashedScenes permanently deletes each scene through HardDeleteScene (scene, generated
// files and the video file, unless it is under a read-only storage path), reporting each
// failure. Only scenes in the trash are deleted: any other scene is reported as failed.
func (s *SceneService) DeleteTrashedScenes(ids []uint) TrashBulkResult {
	result := TrashBulkResult{Succeeded: []uint{}, Failed: []TrashBulkFailure{}}
	for _, id := range uniqueIDs(ids) {
		scene, err := s.Repo.GetByIDIncludingTrashed(id)
		if err != nil {
			result.Failed = append(result.Failed, TrashBulkFailure{SceneID: id, Error: sceneLookupError(err)})
			continue
		}
		if scene.TrashedAt == nil {
			result.Failed = append(result.Failed, TrashBulkFailure{SceneID: id, Error: "scene is not in trash"})
			continue
		}
		if err := s.HardDeleteScene(id); err != nil {
			s.logger.Warn("Failed to permanently delete trashed scene",
				zap.Uint("scene_id", id),
				zap.Error(err),
			)
			result.Failed = append(result.Failed, TrashBulkFailure{SceneID: id, Error: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, id)
		result.Size += scene.Size
	}
	return result
}

func sceneLookupError(err error) string {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "scene not found"
	}
	return fmt.Sprintf("failed to get scene: %v", err)
}
