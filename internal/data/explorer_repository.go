package data

import (
	"fmt"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// buildFullPath constructs a full path by joining a storage base path with a
// relative folder path. It uses string concatenation instead of filepath.Join
// to preserve the original storage path format (e.g. a "./data/videos" prefix).
// filepath.Join calls filepath.Clean which strips prefixes like "./" causing
// LIKE pattern mismatches against stored_path values in the database.
func buildFullPath(basePath, relativePath string) string {
	base := strings.TrimRight(basePath, string(filepath.Separator))
	if relativePath == "" || relativePath == "/" {
		return base + string(filepath.Separator)
	}
	rel := strings.Trim(relativePath, string(filepath.Separator))
	return base + string(filepath.Separator) + rel + string(filepath.Separator)
}

// ExplorerRepository provides folder-based scene access
type ExplorerRepository interface {
	GetStoragePathsWithCounts() ([]StoragePathWithCount, error)
	GetScenesByFolder(storagePathID uint, folderPath string, page, limit int) ([]Scene, int64, error)
	GetSubfolders(storagePathID uint, parentPath string) ([]FolderInfo, error)
	GetSceneIDsByFolder(storagePathID uint, folderPath string, recursive bool) ([]uint, error)
	GetSceneCountByStoragePath(storagePathID uint) (int64, error)
}

type ExplorerRepositoryImpl struct {
	DB *gorm.DB
}

func NewExplorerRepository(db *gorm.DB) *ExplorerRepositoryImpl {
	return &ExplorerRepositoryImpl{DB: db}
}

// GetStoragePathsWithCounts returns all storage paths with their scene counts
func (r *ExplorerRepositoryImpl) GetStoragePathsWithCounts() ([]StoragePathWithCount, error) {
	var results []StoragePathWithCount

	err := r.DB.
		Table("storage_paths").
		Select("storage_paths.*, COALESCE(COUNT(scenes.id), 0) as scene_count").
		Joins("LEFT JOIN scenes ON scenes.storage_path_id = storage_paths.id AND scenes.deleted_at IS NULL AND scenes.trashed_at IS NULL").
		Group("storage_paths.id").
		Order("storage_paths.is_default DESC, storage_paths.name ASC").
		Find(&results).Error

	if err != nil {
		return nil, err
	}
	return results, nil
}

// GetScenesByFolder returns scenes in a specific folder (direct children only)
func (r *ExplorerRepositoryImpl) GetScenesByFolder(storagePathID uint, folderPath string, page, limit int) ([]Scene, int64, error) {
	var scenes []Scene
	var total int64

	offset := (page - 1) * limit

	// Get the storage path to build the full path pattern
	var storagePath StoragePath
	if err := r.DB.First(&storagePath, storagePathID).Error; err != nil {
		return nil, 0, err
	}

	// Build the full folder path (avoid filepath.Join which strips "./" prefixes)
	fullPath := buildFullPath(storagePath.Path, folderPath)

	// Query for scenes directly in this folder (not in subfolders)
	// Match scenes where stored_path starts with fullPath but has no more path separators after that
	// Exclude trashed scenes
	baseQuery := r.DB.Model(&Scene{}).
		Where("storage_path_id = ?", storagePathID).
		Where("stored_path LIKE ?", fullPath+"%").
		Where("stored_path NOT LIKE ?", fullPath+"%"+string(filepath.Separator)+"%").
		Where("trashed_at IS NULL")

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := baseQuery.
		Limit(limit).
		Offset(offset).
		Order("title ASC, original_filename ASC").
		Find(&scenes).Error; err != nil {
		return nil, 0, err
	}

	return scenes, total, nil
}

// GetSubfolders returns unique subfolders within a folder using SQL aggregation
// This is more efficient than loading all videos into memory for large folders
func (r *ExplorerRepositoryImpl) GetSubfolders(storagePathID uint, parentPath string) ([]FolderInfo, error) {
	// Get the storage path
	var storagePath StoragePath
	if err := r.DB.First(&storagePath, storagePathID).Error; err != nil {
		return nil, err
	}

	// Build the full parent path (avoid filepath.Join which strips "./" prefixes)
	fullParentPath := buildFullPath(storagePath.Path, parentPath)

	// Use SQL to extract subfolder names and aggregate scene counts, duration, and size
	// SUBSTRING extracts the relative path after the parent
	// SPLIT_PART gets the first component (immediate subfolder)
	// We filter to only include paths that have a '/' after the parent (i.e., are in subfolders)
	type folderResult struct {
		FolderName    string `gorm:"column:folder_name"`
		SceneCount    int64  `gorm:"column:scene_count"`
		TotalDuration int64  `gorm:"column:total_duration"`
		TotalSize     int64  `gorm:"column:total_size"`
	}

	var results []folderResult
	pathLen := len(fullParentPath)

	// Raw SQL for efficiency - uses PostgreSQL string functions
	// Use a subquery to compute folder_name first, then GROUP BY in outer query
	// Note: pathLen is embedded directly in SQL since SUBSTRING FROM requires a literal integer
	// This is safe as pathLen is derived from database values, not user input
	query := fmt.Sprintf(`
		SELECT folder_name,
		       COUNT(*) as scene_count,
		       COALESCE(SUM(duration), 0) as total_duration,
		       COALESCE(SUM(size), 0) as total_size
		FROM (
			SELECT SPLIT_PART(SUBSTRING(stored_path FROM %d), '/', 1) as folder_name,
			       duration,
			       size
			FROM scenes
			WHERE storage_path_id = ?
			  AND stored_path LIKE ?
			  AND POSITION('/' IN SUBSTRING(stored_path FROM %d)) > 0
			  AND deleted_at IS NULL
			  AND trashed_at IS NULL
		) AS subq
		WHERE folder_name != ''
		GROUP BY folder_name
		ORDER BY LOWER(folder_name)
	`, pathLen+1, pathLen+1)
	err := r.DB.Raw(query, storagePathID, fullParentPath+"%").Scan(&results).Error

	if err != nil {
		return nil, err
	}

	// Build the result with proper paths
	folders := make([]FolderInfo, 0, len(results))
	for _, r := range results {
		var folderFullPath string
		if parentPath == "" || parentPath == "/" {
			folderFullPath = r.FolderName
		} else {
			folderFullPath = filepath.Join(parentPath, r.FolderName)
		}

		folders = append(folders, FolderInfo{
			Name:          r.FolderName,
			Path:          folderFullPath,
			SceneCount:    r.SceneCount,
			TotalDuration: r.TotalDuration,
			TotalSize:     r.TotalSize,
		})
	}

	return folders, nil
}

// GetSceneIDsByFolder returns scene IDs in a folder, optionally recursive
func (r *ExplorerRepositoryImpl) GetSceneIDsByFolder(storagePathID uint, folderPath string, recursive bool) ([]uint, error) {
	// Get the storage path
	var storagePath StoragePath
	if err := r.DB.First(&storagePath, storagePathID).Error; err != nil {
		return nil, err
	}

	// Build the full folder path (avoid filepath.Join which strips "./" prefixes)
	fullPath := buildFullPath(storagePath.Path, folderPath)

	var ids []uint
	query := r.DB.Model(&Scene{}).
		Where("storage_path_id = ?", storagePathID).
		Where("stored_path LIKE ?", fullPath+"%").
		Where("trashed_at IS NULL")

	if !recursive {
		// Only direct children
		query = query.Where("stored_path NOT LIKE ?", fullPath+"%"+string(filepath.Separator)+"%")
	}

	if err := query.Pluck("id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// GetSceneCountByStoragePath returns total scene count for a storage path
func (r *ExplorerRepositoryImpl) GetSceneCountByStoragePath(storagePathID uint) (int64, error) {
	var count int64
	err := r.DB.Model(&Scene{}).
		Where("storage_path_id = ?", storagePathID).
		Where("trashed_at IS NULL").
		Count(&count).Error
	return count, err
}
