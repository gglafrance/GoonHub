package data

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// PrivacyRepository stores the explicit privacy choices of the users (user_privacy_settings).
// An item without a row has the catalog default (core/privacy_catalog.go): the repository
// knows nothing of the catalog.
type PrivacyRepository interface {
	// ListSettings returns the explicit choices of a user.
	ListSettings(ctx context.Context, userID uint) ([]UserPrivacySetting, error)
	// AudiencesFor returns the explicit audience of an item for each owner that has one
	// (owners without a row are absent from the map).
	AudiencesFor(ctx context.Context, ownerIDs []uint, item string) (map[uint]string, error)
	// SaveSettings sets the audience of the items in set and deletes the choices of the items
	// in reset (back to the default), in one transaction.
	SaveSettings(ctx context.Context, userID uint, set map[string]string, reset []string) error
}

type PrivacyRepositoryImpl struct {
	DB *gorm.DB
}

func NewPrivacyRepository(db *gorm.DB) *PrivacyRepositoryImpl {
	return &PrivacyRepositoryImpl{DB: db}
}

func (r *PrivacyRepositoryImpl) ListSettings(ctx context.Context, userID uint) ([]UserPrivacySetting, error) {
	var rows []UserPrivacySetting
	if err := r.DB.WithContext(ctx).Where("user_id = ?", userID).Order("item").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list privacy settings: %w", err)
	}
	return rows, nil
}

func (r *PrivacyRepositoryImpl) AudiencesFor(ctx context.Context, ownerIDs []uint, item string) (map[uint]string, error) {
	out := make(map[uint]string)
	if len(ownerIDs) == 0 {
		return out, nil
	}
	var rows []UserPrivacySetting
	if err := r.DB.WithContext(ctx).Where("item = ? AND user_id = ANY(?)", item, uintsToInt64Array(ownerIDs)).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to read privacy settings: %w", err)
	}
	for _, row := range rows {
		out[row.UserID] = row.Audience
	}
	return out, nil
}

func (r *PrivacyRepositoryImpl) SaveSettings(ctx context.Context, userID uint, set map[string]string, reset []string) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for item, audience := range set {
			if err := tx.Exec(`INSERT INTO user_privacy_settings (user_id, item, audience, updated_at) VALUES (?, ?, ?, NOW())
				ON CONFLICT (user_id, item) DO UPDATE SET audience = EXCLUDED.audience, updated_at = NOW()`, userID, item, audience).Error; err != nil {
				return fmt.Errorf("failed to save privacy setting %s: %w", item, err)
			}
		}
		if len(reset) > 0 {
			if err := tx.Exec(`DELETE FROM user_privacy_settings WHERE user_id = ? AND item IN ?`, userID, reset).Error; err != nil {
				return fmt.Errorf("failed to reset privacy settings: %w", err)
			}
		}
		return nil
	})
}
