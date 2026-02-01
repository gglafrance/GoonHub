package data

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Actor struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	UUID            uuid.UUID      `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	Name            string     `gorm:"size:255;not null" json:"name"`
	ImageURL        string     `gorm:"size:512" json:"image_url"`
	Gender          string     `gorm:"size:50" json:"gender"`
	Birthday        *time.Time `json:"birthday"`
	DateOfDeath     *time.Time `json:"date_of_death"`
	Astrology       string     `gorm:"size:50" json:"astrology"`
	Birthplace      string     `gorm:"size:255" json:"birthplace"`
	Ethnicity       string     `gorm:"size:100" json:"ethnicity"`
	Nationality     string     `gorm:"size:100" json:"nationality"`
	CareerStartYear *int       `json:"career_start_year"`
	CareerEndYear   *int       `json:"career_end_year"`
	HeightCm        *int       `json:"height_cm"`
	WeightKg        *int       `json:"weight_kg"`
	Measurements    string     `gorm:"size:50" json:"measurements"`
	Cupsize         string     `gorm:"size:10" json:"cupsize"`
	HairColor       string     `gorm:"size:50" json:"hair_color"`
	EyeColor        string     `gorm:"size:50" json:"eye_color"`
	Tattoos         string     `gorm:"type:text" json:"tattoos"`
	Piercings       string     `gorm:"type:text" json:"piercings"`
	FakeBoobs       bool       `gorm:"not null;default:false" json:"fake_boobs"`
	SameSexOnly     bool       `gorm:"not null;default:false" json:"same_sex_only"`
}

// BeforeCreate generates a UUID if not set
func (a *Actor) BeforeCreate(tx *gorm.DB) error {
	if a.UUID == uuid.Nil {
		a.UUID = uuid.New()
	}
	return nil
}

type ActorWithCount struct {
	Actor
	SceneCount int64 `json:"scene_count"`
}

type SceneActor struct {
	ID      uint `gorm:"primarykey"`
	SceneID uint `gorm:"not null;column:scene_id"`
	ActorID uint `gorm:"not null"`
}

func (SceneActor) TableName() string {
	return "scene_actors"
}
