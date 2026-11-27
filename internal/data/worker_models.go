package data

import (
	"time"

	"github.com/lib/pq"
)

// Worker is a registered remote worker (goonhub-worker, docs/REMOTE_WORKERS.md).
type Worker struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	Name           string         `gorm:"not null;size:100" json:"name"`
	TokenHash      *string        `gorm:"size:64" json:"-"`
	Enabled        bool           `gorm:"not null;default:true" json:"enabled"`
	Draining       bool           `gorm:"not null;default:false" json:"draining"`
	ConnectionMode string         `gorm:"not null;size:10;default:'pull'" json:"connection_mode"`
	PushURL        string         `gorm:"not null;size:500;default:''" json:"push_url"`
	PushToken      string         `gorm:"not null;size:200;default:''" json:"-"`
	SessionID      string         `gorm:"not null;size:64;default:''" json:"-"`
	ServerURL      string         `gorm:"not null;size:500;default:''" json:"server_url"`
	Capabilities   []byte         `gorm:"type:jsonb;not null;default:'{}'" json:"-"`
	PathMappings   []byte         `gorm:"type:jsonb;not null;default:'[]'" json:"-"`
	AllowedPhases  pq.StringArray `gorm:"type:text[];not null" json:"allowed_phases"`
	MaxConcurrency int            `gorm:"not null;default:0" json:"max_concurrency"`
	Version        string         `gorm:"not null;size:50;default:''" json:"version"`
	Hostname       string         `gorm:"not null;size:255;default:''" json:"hostname"`
	OS             string         `gorm:"column:os;not null;size:20;default:''" json:"os"`
	Arch           string         `gorm:"not null;size:20;default:''" json:"arch"`
	FreeDiskBytes  int64          `gorm:"not null;default:0" json:"free_disk_bytes"`
	WorkerDraining bool           `gorm:"not null;default:false" json:"worker_draining"`
	LastSeenAt     *time.Time     `json:"last_seen_at"`
	RegisteredAt   *time.Time     `json:"registered_at"`
	JobsCompleted  int64          `gorm:"not null;default:0" json:"jobs_completed"`
	JobsFailed     int64          `gorm:"not null;default:0" json:"jobs_failed"`
	LeasesExpired  int64          `gorm:"not null;default:0" json:"leases_expired"`
	LeasesReleased int64          `gorm:"not null;default:0" json:"leases_released"`
	LastError      string         `gorm:"type:text;not null;default:''" json:"last_error"`
	LastErrorAt    *time.Time     `json:"last_error_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (Worker) TableName() string {
	return "workers"
}

// Worker statistics counters (WorkerRepository.IncrementCounter).
const (
	WorkerCounterCompleted = "jobs_completed"
	WorkerCounterFailed    = "jobs_failed"
	WorkerCounterExpired   = "leases_expired"
	WorkerCounterReleased  = "leases_released"
)

// WorkerSettings is the singleton row of the remote worker settings.
type WorkerSettings struct {
	ID           int       `gorm:"primaryKey" json:"-"`
	MediaSecret  string    `gorm:"not null;size:128;default:''" json:"-"`
	PhaseRouting []byte    `gorm:"type:jsonb;not null;default:'{}'" json:"-"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (WorkerSettings) TableName() string {
	return "worker_settings"
}
