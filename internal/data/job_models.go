package data

import (
	"time"
)

type JobHistory struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	JobID        string     `gorm:"uniqueIndex;not null;size:36" json:"job_id"`
	VideoID      uint       `gorm:"not null" json:"video_id"`
	VideoTitle   string     `gorm:"not null;size:255;default:''" json:"video_title"`
	Phase        string     `gorm:"not null;size:20" json:"phase"`
	Status       string     `gorm:"not null;size:20;default:'running'" json:"status"`
	ErrorMessage *string    `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt    time.Time  `gorm:"not null;default:now()" json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at"`
	RetryCount   int        `gorm:"not null;default:0" json:"retry_count"`
	MaxRetries   int        `gorm:"not null;default:0" json:"max_retries"`
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`
	Progress     int        `gorm:"not null;default:0" json:"progress"`
	IsRetryable  bool       `gorm:"not null;default:true" json:"is_retryable"`
}

func (JobHistory) TableName() string {
	return "job_history"
}

type DLQEntry struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	JobID         string     `gorm:"uniqueIndex;not null;size:36" json:"job_id"`
	VideoID       uint       `gorm:"not null" json:"video_id"`
	VideoTitle    string     `gorm:"not null;size:255;default:''" json:"video_title"`
	Phase         string     `gorm:"not null;size:20" json:"phase"`
	OriginalError string     `gorm:"type:text;not null" json:"original_error"`
	FailureCount  int        `gorm:"not null;default:1" json:"failure_count"`
	LastError     string     `gorm:"type:text;not null" json:"last_error"`
	Status        string     `gorm:"not null;size:20;default:'pending_review'" json:"status"`
	CreatedAt     time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"not null;default:now()" json:"updated_at"`
	AbandonedAt   *time.Time `json:"abandoned_at,omitempty"`
}

func (DLQEntry) TableName() string {
	return "dead_letter_queue"
}

type RetryConfigRecord struct {
	ID                  int       `gorm:"primaryKey" json:"id"`
	Phase               string    `gorm:"uniqueIndex;not null;size:20" json:"phase"`
	MaxRetries          int       `gorm:"not null;default:3" json:"max_retries"`
	InitialDelaySeconds int       `gorm:"not null;default:30" json:"initial_delay_seconds"`
	MaxDelaySeconds     int       `gorm:"not null;default:3600" json:"max_delay_seconds"`
	BackoffFactor       float64   `gorm:"type:decimal(3,1);not null;default:2.0" json:"backoff_factor"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (RetryConfigRecord) TableName() string {
	return "retry_config"
}
