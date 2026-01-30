package data

import (
	"fmt"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// ExplorerRepository provides folder-based video access
type ExplorerRepository interface {
	GetStoragePathsWithCounts() ([]StoragePathWithCount, error)
	GetVideosByFolder(storagePathID uint, folderPath string, page, limit int) ([]Video, int64, error)
	GetSubfolders(storagePathID uint, parentPath string) ([]FolderInfo, error)
	GetVideoIDsByFolder(storagePathID uint, folderPath string, recursive bool) ([]uint, error)
	GetVideoCountByStoragePath(storagePathID uint) (int64, error)
}

type ExplorerRepositoryImpl struct {
	DB *gorm.DB
}

func NewExplorerRepository(db *gorm.DB) *ExplorerRepositoryImpl {
	return &ExplorerRepositoryImpl{DB: db}
}

// GetStoragePathsWithCounts returns all storage paths with their video counts
func (r *ExplorerRepositoryImpl) GetStoragePathsWithCounts() ([]StoragePathWithCount, error) {
	var results []StoragePathWithCount

	err := r.DB.
		Table("storage_paths").
		Select("storage_paths.*, COALESCE(COUNT(videos.id), 0) as video_count").
		Joins("LEFT JOIN videos ON videos.storage_path_id = storage_paths.id AND videos.deleted_at IS NULL").
		Group("storage_paths.id").
		Order("storage_paths.is_default DESC, storage_paths.name ASC").
		Find(&results).Error

	if err != nil {
		return nil, err
	}
	return results, nil
}

// GetVideosByFolder returns videos in a specific folder (direct children only)
func (r *ExplorerRepositoryImpl) GetVideosByFolder(storagePathID uint, folderPath string, page, limit int) ([]Video, int64, error) {
	var videos []Video
	var total int64

	offset := (page - 1) * limit

	// Get the storage path to build the full path pattern
	var storagePath StoragePath
	if err := r.DB.First(&storagePath, storagePathID).Error; err != nil {
		return nil, 0, err
	}

	// Build the full folder path
	var fullPath string
	if folderPath == "" || folderPath == "/" {
		fullPath = storagePath.Path
	} else {
		fullPath = filepath.Join(storagePath.Path, folderPath)
	}

	// Ensure path has trailing separator for matching
	if !strings.HasSuffix(fullPath, string(filepath.Separator)) {
		fullPath = fullPath + string(filepath.Separator)
	}

	// Query for videos directly in this folder (not in subfolders)
	// Match videos where stored_path starts with fullPath but has no more path separators after that
	baseQuery := r.DB.Model(&Video{}).
		Where("storage_path_id = ?", storagePathID).
		Where("stored_path LIKE ?", fullPath+"%").
		Where("stored_path NOT LIKE ?", fullPath+"%"+string(filepath.Separator)+"%")

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := baseQuery.
		Limit(limit).
		Offset(offset).
		Order("title ASC, original_filename ASC").
		Find(&videos).Error; err != nil {
		return nil, 0, err
	}

	return videos, total, nil
}

// GetSubfolders returns unique subfolders within a folder using SQL aggregation
// This is more efficient than loading all videos into memory for large folders
func (r *ExplorerRepositoryImpl) GetSubfolders(storagePathID uint, parentPath string) ([]FolderInfo, error) {
	// Get the storage path
	var storagePath StoragePath
	if err := r.DB.First(&storagePath, storagePathID).Error; err != nil {
		return nil, err
	}

	// Build the full parent path
	var fullParentPath string
	if parentPath == "" || parentPath == "/" {
		fullParentPath = storagePath.Path
	} else {
		fullParentPath = filepath.Join(storagePath.Path, parentPath)
	}

	// Ensure path has trailing separator
	if !strings.HasSuffix(fullParentPath, string(filepath.Separator)) {
		fullParentPath = fullParentPath + string(filepath.Separator)
	}

	// Use SQL to extract subfolder names and aggregate video counts, duration, and size
	// SUBSTRING extracts the relative path after the parent
	// SPLIT_PART gets the first component (immediate subfolder)
	// We filter to only include paths that have a '/' after the parent (i.e., are in subfolders)
	type folderResult struct {
		FolderName    string `gorm:"column:folder_name"`
		VideoCount    int64  `gorm:"column:video_count"`
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
		       COUNT(*) as video_count,
		       COALESCE(SUM(duration), 0) as total_duration,
		       COALESCE(SUM(size), 0) as total_size
		FROM (
			SELECT SPLIT_PART(SUBSTRING(stored_path FROM %d), '/', 1) as folder_name,
			       duration,
			       size
			FROM videos
			WHERE storage_path_id = ?
			  AND stored_path LIKE ?
			  AND POSITION('/' IN SUBSTRING(stored_path FROM %d)) > 0
			  AND deleted_at IS NULL
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
			VideoCount:    r.VideoCount,
			TotalDuration: r.TotalDuration,
			TotalSize:     r.TotalSize,
		})
	}

	return folders, nil
}

// GetVideoIDsByFolder returns video IDs in a folder, optionally recursive
func (r *ExplorerRepositoryImpl) GetVideoIDsByFolder(storagePathID uint, folderPath string, recursive bool) ([]uint, error) {
	// Get the storage path
	var storagePath StoragePath
	if err := r.DB.First(&storagePath, storagePathID).Error; err != nil {
		return nil, err
	}

	// Build the full folder path
	var fullPath string
	if folderPath == "" || folderPath == "/" {
		fullPath = storagePath.Path
	} else {
		fullPath = filepath.Join(storagePath.Path, folderPath)
	}

	// Ensure path has trailing separator
	if !strings.HasSuffix(fullPath, string(filepath.Separator)) {
		fullPath = fullPath + string(filepath.Separator)
	}

	var ids []uint
	query := r.DB.Model(&Video{}).
		Where("storage_path_id = ?", storagePathID).
		Where("stored_path LIKE ?", fullPath+"%")

	if !recursive {
		// Only direct children
		query = query.Where("stored_path NOT LIKE ?", fullPath+"%"+string(filepath.Separator)+"%")
	}

	if err := query.Pluck("id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// GetVideoCountByStoragePath returns total video count for a storage path
func (r *ExplorerRepositoryImpl) GetVideoCountByStoragePath(storagePathID uint) (int64, error) {
	var count int64
	err := r.DB.Model(&Video{}).
		Where("storage_path_id = ?", storagePathID).
		Count(&count).Error
	return count, err
}
