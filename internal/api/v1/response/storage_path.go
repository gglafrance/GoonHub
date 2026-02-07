package response

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// DiskUsageResponse represents filesystem usage stats for a storage path.
type DiskUsageResponse struct {
	TotalBytes uint64  `json:"total_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	FreeBytes  uint64  `json:"free_bytes"`
	UsedPct    float64 `json:"used_pct"`
}

// StoragePathWithUsage combines a storage path with optional disk usage info.
type StoragePathWithUsage struct {
	ID        uint               `json:"id"`
	Name      string             `json:"name"`
	Path      string             `json:"path"`
	IsDefault bool               `json:"is_default"`
	CreatedAt string             `json:"created_at"`
	UpdatedAt string             `json:"updated_at"`
	DiskUsage *DiskUsageResponse `json:"disk_usage"`
}

// ToStoragePathsWithUsage converts storage paths and a usage map into response DTOs.
func ToStoragePathsWithUsage(paths []data.StoragePath, usageMap map[uint]*core.DiskUsage) []StoragePathWithUsage {
	result := make([]StoragePathWithUsage, len(paths))
	for i, p := range paths {
		var usage *DiskUsageResponse
		if du := usageMap[p.ID]; du != nil {
			usage = &DiskUsageResponse{
				TotalBytes: du.TotalBytes,
				UsedBytes:  du.UsedBytes,
				FreeBytes:  du.FreeBytes,
				UsedPct:    du.UsedPct,
			}
		}
		result[i] = StoragePathWithUsage{
			ID:        p.ID,
			Name:      p.Name,
			Path:      p.Path,
			IsDefault: p.IsDefault,
			CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			DiskUsage: usage,
		}
	}
	return result
}
