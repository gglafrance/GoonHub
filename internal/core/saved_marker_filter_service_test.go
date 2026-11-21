package core

import (
	"strings"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newTestSavedMarkerFilterService(t *testing.T) (*SavedMarkerFilterService, *mocks.MockSavedMarkerFilterRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSavedMarkerFilterRepository(ctrl)
	return NewSavedMarkerFilterService(repo, zap.NewNop()), repo
}

func TestSavedMarkerFilterCreate_Validation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		values  data.MarkerFilterValues
		wantErr string
	}{
		{"empty name", "  ", data.MarkerFilterValues{}, "name is required"},
		{"name too long", strings.Repeat("a", 101), data.MarkerFilterValues{}, "at most 100"},
		{"bad sort", "x", data.MarkerFilterValues{Sort: "shiny"}, "invalid sort"},
		{"bad view", "x", data.MarkerFilterValues{View: "list"}, "view must be"},
		{"too many labels", "x", data.MarkerFilterValues{Labels: func() []string {
			l := make([]string, 51)
			for i := range l {
				l[i] = strings.Repeat("l", i+1)
			}
			return l
		}()}, "at most 50"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestSavedMarkerFilterService(t)
			_, err := svc.Create(1, tt.input, tt.values)
			if !apperrors.IsValidation(err) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected validation error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSavedMarkerFilterCreate_NormalizesValues(t *testing.T) {
	svc, repo := newTestSavedMarkerFilterService(t)
	var saved *data.SavedMarkerFilter
	repo.EXPECT().Create(gomock.Any()).DoAndReturn(func(f *data.SavedMarkerFilter) error {
		saved = f
		return nil
	})

	_, err := svc.Create(7, "  Cum suck ", data.MarkerFilterValues{
		MarkerTags: []string{" Cum", "Suck", "Cum", ""},
		Sort:       "random",
		View:       "all",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.UserID != 7 || saved.Name != "Cum suck" || len(saved.Filters.MarkerTags) != 2 {
		t.Fatalf("unexpected saved filter: %+v", saved)
	}
}

func TestSavedMarkerFilterUpdate_OtherUserIsNotFound(t *testing.T) {
	svc, repo := newTestSavedMarkerFilterService(t)
	id := uuid.New()
	repo.EXPECT().GetByUUID(id.String()).Return(&data.SavedMarkerFilter{ID: 3, UUID: id, UserID: 2}, nil)

	name := "renamed"
	if _, err := svc.Update(1, id.String(), &name, nil); !apperrors.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestSavedMarkerFilterDelete(t *testing.T) {
	svc, repo := newTestSavedMarkerFilterService(t)
	id := uuid.New()
	repo.EXPECT().GetByUUID(id.String()).Return(&data.SavedMarkerFilter{ID: 3, UUID: id, UserID: 1}, nil)
	repo.EXPECT().Delete(uint(3)).Return(nil)
	if err := svc.Delete(1, id.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repo.EXPECT().GetByUUID("missing").Return(nil, gorm.ErrRecordNotFound)
	if err := svc.Delete(1, "missing"); !apperrors.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}
