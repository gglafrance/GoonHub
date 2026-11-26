package data

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NavLayout is a user's layout of the two header bars (docs/NAVIGATION.md). The entries of a
// zone are item ids or folder references ("folder:<id>"); the items of a folder are item ids.
// An item that is neither placed nor hidden goes to its default place.
type NavLayout struct {
	Version int           `json:"version"`
	Bars    NavLayoutBars `json:"bars"`
	Folders []NavFolder   `json:"folders"`
	Hidden  []string      `json:"hidden"`
}

// NavLayoutBars are the two header bars: top (logo row) and nav (the links row).
type NavLayoutBars struct {
	Top NavBarZones `json:"top"`
	Nav NavBarZones `json:"nav"`
}

// NavBarZones are the three zones of a bar, each an ordered list of entries.
type NavBarZones struct {
	Left   []string `json:"left"`
	Center []string `json:"center"`
	Right  []string `json:"right"`
}

// NavFolder is a user-created group of items shown as a dropdown button.
type NavFolder struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Icon  string   `json:"icon"`
	Items []string `json:"items"`
}

// Normalize replaces the nil lists with empty ones (the JSON always has arrays).
func (l *NavLayout) Normalize() {
	for _, z := range []*NavBarZones{&l.Bars.Top, &l.Bars.Nav} {
		if z.Left == nil {
			z.Left = []string{}
		}
		if z.Center == nil {
			z.Center = []string{}
		}
		if z.Right == nil {
			z.Right = []string{}
		}
	}
	if l.Folders == nil {
		l.Folders = []NavFolder{}
	}
	for i := range l.Folders {
		if l.Folders[i].Items == nil {
			l.Folders[i].Items = []string{}
		}
	}
	if l.Hidden == nil {
		l.Hidden = []string{}
	}
}

// Value implements the driver.Valuer interface for JSONB storage
func (l NavLayout) Value() (driver.Value, error) {
	l.Normalize()
	return json.Marshal(l)
}

// Scan implements the sql.Scanner interface for JSONB retrieval
func (l *NavLayout) Scan(value any) error {
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return errors.New("failed to scan NavLayout: expected []byte")
	}
	*l = NavLayout{}
	if err := json.Unmarshal(raw, l); err != nil {
		return err
	}
	l.Normalize()
	return nil
}

// UserNavLayout is the stored layout of a user (user_nav_layouts). No row = default layout.
type UserNavLayout struct {
	UserID    uint      `gorm:"primaryKey;autoIncrement:false"`
	Layout    NavLayout `gorm:"type:jsonb;not null"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`
}

func (UserNavLayout) TableName() string { return "user_nav_layouts" }

// NavLayoutRepository stores the header layouts of the users.
type NavLayoutRepository interface {
	// Get returns the stored layout of a user, nil when the user has none (default layout).
	Get(ctx context.Context, userID uint) (*NavLayout, error)
	// Save stores the layout of a user (insert or replace).
	Save(ctx context.Context, userID uint, layout NavLayout) error
	// Delete removes the layout of a user (back to the default layout).
	Delete(ctx context.Context, userID uint) error
}

type NavLayoutRepositoryImpl struct {
	DB *gorm.DB
}

func NewNavLayoutRepository(db *gorm.DB) *NavLayoutRepositoryImpl {
	return &NavLayoutRepositoryImpl{DB: db}
}

func (r *NavLayoutRepositoryImpl) Get(ctx context.Context, userID uint) (*NavLayout, error) {
	var row UserNavLayout
	err := r.DB.WithContext(ctx).Where("user_id = ?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read the nav layout: %w", err)
	}
	return &row.Layout, nil
}

func (r *NavLayoutRepositoryImpl) Save(ctx context.Context, userID uint, layout NavLayout) error {
	row := UserNavLayout{UserID: userID, Layout: layout, UpdatedAt: time.Now()}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"layout", "updated_at"}),
	}).Create(&row).Error; err != nil {
		return fmt.Errorf("failed to save the nav layout: %w", err)
	}
	return nil
}

func (r *NavLayoutRepositoryImpl) Delete(ctx context.Context, userID uint) error {
	if err := r.DB.WithContext(ctx).Where("user_id = ?", userID).Delete(&UserNavLayout{}).Error; err != nil {
		return fmt.Errorf("failed to reset the nav layout: %w", err)
	}
	return nil
}
