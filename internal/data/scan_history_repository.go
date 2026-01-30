package data

import (
	"time"

	"gorm.io/gorm"
)

type ScanHistory struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	Status        string     `gorm:"not null;default:'running'" json:"status"`
	StartedAt     time.Time  `gorm:"not null;default:now()" json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	PathsScanned  int        `gorm:"not null;default:0" json:"paths_scanned"`
	FilesFound    int        `gorm:"not null;default:0" json:"files_found"`
	VideosAdded   int        `gorm:"not null;default:0" json:"videos_added"`
	VideosSkipped int        `gorm:"not null;default:0" json:"videos_skipped"`
	VideosRemoved int        `gorm:"not null;default:0" json:"videos_removed"`
	VideosMoved   int        `gorm:"not null;default:0" json:"videos_moved"`
	Errors        int        `gorm:"not null;default:0" json:"errors"`
	ErrorMessage  *string    `gorm:"type:text" json:"error_message,omitempty"`
	CurrentPath   *string    `gorm:"size:500" json:"current_path,omitempty"`
	CurrentFile   *string    `gorm:"size:500" json:"current_file,omitempty"`
	CreatedAt     time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

func (ScanHistory) TableName() string {
	return "scan_history"
}

type ScanHistoryRepository interface {
	Create(scan *ScanHistory) error
	Update(scan *ScanHistory) error
	GetByID(id uint) (*ScanHistory, error)
	GetLatest() (*ScanHistory, error)
	GetRunning() (*ScanHistory, error)
	List(page, limit int) ([]ScanHistory, int64, error)
	MarkInterruptedAsFailedOnStartup() error
}

type ScanHistoryRepositoryImpl struct {
	DB *gorm.DB
}

func NewScanHistoryRepository(db *gorm.DB) *ScanHistoryRepositoryImpl {
	return &ScanHistoryRepositoryImpl{DB: db}
}

func (r *ScanHistoryRepositoryImpl) Create(scan *ScanHistory) error {
	return r.DB.Create(scan).Error
}

func (r *ScanHistoryRepositoryImpl) Update(scan *ScanHistory) error {
	return r.DB.Save(scan).Error
}

func (r *ScanHistoryRepositoryImpl) GetByID(id uint) (*ScanHistory, error) {
	var scan ScanHistory
	err := r.DB.First(&scan, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &scan, nil
}

func (r *ScanHistoryRepositoryImpl) GetLatest() (*ScanHistory, error) {
	var scan ScanHistory
	err := r.DB.Order("started_at DESC").First(&scan).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &scan, nil
}

func (r *ScanHistoryRepositoryImpl) GetRunning() (*ScanHistory, error) {
	var scan ScanHistory
	err := r.DB.Where("status = ?", "running").First(&scan).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &scan, nil
}

func (r *ScanHistoryRepositoryImpl) List(page, limit int) ([]ScanHistory, int64, error) {
	var scans []ScanHistory
	var total int64

	offset := (page - 1) * limit

	if err := r.DB.Model(&ScanHistory{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.DB.Limit(limit).Offset(offset).Order("started_at DESC").Find(&scans).Error; err != nil {
		return nil, 0, err
	}

	return scans, total, nil
}

func (r *ScanHistoryRepositoryImpl) MarkInterruptedAsFailedOnStartup() error {
	now := time.Now()
	errMsg := "Scan interrupted by server restart"
	return r.DB.Model(&ScanHistory{}).
		Where("status = ?", "running").
		Updates(map[string]any{
			"status":        "failed",
			"completed_at":  now,
			"error_message": errMsg,
			"current_path":  nil,
			"current_file":  nil,
		}).Error
}
