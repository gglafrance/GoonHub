package data

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// SceneOrigin enum values
const (
	SceneOriginWeb      = "web"
	SceneOriginDVD      = "dvd"
	SceneOriginPersonal = "personal"
	SceneOriginStash    = "stash"
	SceneOriginUnknown  = "unknown"
)

// SceneType enum values
const (
	SceneTypeStandard     = "standard"
	SceneTypeJAV          = "jav"
	SceneTypeHentai       = "hentai"
	SceneTypeAmateur      = "amateur"
	SceneTypeProfessional = "professional"
	SceneTypeVR           = "vr"
	SceneTypeCompilation  = "compilation"
	SceneTypePMV          = "pmv"
)

// ValidSceneOrigins returns all valid origin values
func ValidSceneOrigins() []string {
	return []string{
		SceneOriginWeb,
		SceneOriginDVD,
		SceneOriginPersonal,
		SceneOriginStash,
		SceneOriginUnknown,
	}
}

// ValidSceneTypes returns all valid type values
func ValidSceneTypes() []string {
	return []string{
		SceneTypeStandard,
		SceneTypeJAV,
		SceneTypeHentai,
		SceneTypeAmateur,
		SceneTypeProfessional,
		SceneTypeVR,
		SceneTypeCompilation,
		SceneTypePMV,
	}
}

// IsValidSceneOrigin checks if the given origin is valid
func IsValidSceneOrigin(origin string) bool {
	for _, v := range ValidSceneOrigins() {
		if v == origin {
			return true
		}
	}
	return false
}

// IsValidSceneType checks if the given type is valid
func IsValidSceneType(sceneType string) bool {
	for _, v := range ValidSceneTypes() {
		if v == sceneType {
			return true
		}
	}
	return false
}

type Scene struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	Title            string         `json:"title"`
	OriginalFilename string         `json:"original_filename"`
	StoredPath       string         `json:"stored_path"`
	Size             int64          `json:"size"`
	ViewCount        int64          `json:"view_count"`
	Duration         int            `json:"duration"`
	Width            int            `json:"width"`
	Height           int            `json:"height"`
	ThumbnailPath    string         `json:"thumbnail_path"`
	SpriteSheetPath  string         `json:"sprite_sheet_path"`
	VttPath          string         `json:"vtt_path"`
	SpriteSheetCount int            `json:"sprite_sheet_count"`
	ThumbnailWidth   int            `json:"thumbnail_width"`
	ThumbnailHeight  int            `json:"thumbnail_height"`
	ProcessingStatus string         `json:"processing_status" gorm:"default:'pending'"`
	ProcessingError  string         `json:"processing_error" gorm:"type:text"`
	FileCreatedAt    *time.Time     `json:"file_created_at"`
	Description      string         `json:"description"`
	Studio           string         `json:"studio"`
	Tags             pq.StringArray `json:"tags" gorm:"type:text[]"`
	Actors           pq.StringArray `json:"actors" gorm:"type:text[]"`
	CoverImagePath   string         `json:"cover_image_path"`
	FileHash         string         `json:"file_hash"`
	FrameRate        float64        `json:"frame_rate"`
	BitRate          int64          `json:"bit_rate"`
	VideoCodec       string         `json:"video_codec"`
	AudioCodec       string         `json:"audio_codec"`
	StoragePathID    *uint          `json:"storage_path_id"`
	StudioID         *uint          `json:"studio_id"`
	ReleaseDate      *time.Time     `json:"release_date" gorm:"type:date"`
	PornDBSceneID    string         `json:"porndb_scene_id" gorm:"column:porndb_scene_id"`
	Origin           string         `json:"origin" gorm:"size:100"`
	Type             string         `json:"type" gorm:"size:50"`
	PreviewVideoPath string         `json:"preview_video_path"`
	IsCorrupted       bool           `json:"is_corrupted" gorm:"default:false"`
	TrashedAt         *time.Time     `json:"trashed_at,omitempty" gorm:"index"`
	AudioFingerprint  []byte         `json:"-" gorm:"column:audio_fingerprint"`
	VisualFingerprint []byte         `json:"-" gorm:"column:visual_fingerprint"`
	FingerprintType   *string        `json:"fingerprint_type,omitempty" gorm:"column:fingerprint_type;size:10"`
	FingerprintAt     *time.Time     `json:"fingerprint_at,omitempty" gorm:"column:fingerprint_at"`
}

func (Scene) TableName() string {
	return "scenes"
}

type Tag struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `gorm:"uniqueIndex;not null;size:100" json:"name"`
	Color     string    `gorm:"not null;size:7;default:'#6B7280'" json:"color"`
}

type SceneTag struct {
	ID      uint `gorm:"primarykey"`
	SceneID uint `gorm:"not null;column:scene_id"`
	TagID   uint `gorm:"not null"`
}

func (SceneTag) TableName() string {
	return "scene_tags"
}
