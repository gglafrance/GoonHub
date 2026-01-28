package core

import (
	"fmt"
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newTestTagService(t *testing.T) (*TagService, *mocks.MockTagRepository, *mocks.MockVideoRepository) {
	ctrl := gomock.NewController(t)
	tagRepo := mocks.NewMockTagRepository(ctrl)
	videoRepo := mocks.NewMockVideoRepository(ctrl)

	svc := NewTagService(tagRepo, videoRepo, zap.NewNop())
	return svc, tagRepo, videoRepo
}

func TestListTags_Success(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)

	expected := []data.TagWithCount{
		{Tag: data.Tag{ID: 1, Name: "Amateur", Color: "#8B5CF6"}, VideoCount: 3},
		{Tag: data.Tag{ID: 2, Name: "Favorite", Color: "#FF4D4D"}, VideoCount: 5},
	}
	tagRepo.EXPECT().ListWithCounts().Return(expected, nil)

	tags, err := svc.ListTags()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Name != "Amateur" {
		t.Fatalf("expected first tag 'Amateur', got %q", tags[0].Name)
	}
	if tags[0].VideoCount != 3 {
		t.Fatalf("expected first tag video_count 3, got %d", tags[0].VideoCount)
	}
}

func TestCreateTag_Success(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)

	tagRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(tag *data.Tag) error {
		if tag.Name != "NewTag" {
			t.Fatalf("expected name 'NewTag', got %q", tag.Name)
		}
		if tag.Color != "#22C55E" {
			t.Fatalf("expected color '#22C55E', got %q", tag.Color)
		}
		tag.ID = 1
		return nil
	})

	tag, err := svc.CreateTag("NewTag", "#22C55E")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tag.Name != "NewTag" {
		t.Fatalf("expected name 'NewTag', got %q", tag.Name)
	}
}

func TestCreateTag_EmptyName(t *testing.T) {
	svc, _, _ := newTestTagService(t)

	_, err := svc.CreateTag("", "#FF4D4D")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "tag name is required") {
		t.Fatalf("expected 'tag name is required' error, got: %v", err)
	}
}

func TestCreateTag_NameTooLong(t *testing.T) {
	svc, _, _ := newTestTagService(t)

	longName := strings.Repeat("a", 101)
	_, err := svc.CreateTag(longName, "#FF4D4D")
	if err == nil {
		t.Fatal("expected error for long name")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "100 characters or less") {
		t.Fatalf("expected length error, got: %v", err)
	}
}

func TestCreateTag_InvalidColor(t *testing.T) {
	svc, _, _ := newTestTagService(t)

	_, err := svc.CreateTag("Test", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid color")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid color format") {
		t.Fatalf("expected color format error, got: %v", err)
	}
}

func TestCreateTag_EmptyColorDefaulted(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)

	tagRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(tag *data.Tag) error {
		if tag.Color != "#6B7280" {
			t.Fatalf("expected default color '#6B7280', got %q", tag.Color)
		}
		tag.ID = 1
		return nil
	})

	tag, err := svc.CreateTag("Test", "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tag.Color != "#6B7280" {
		t.Fatalf("expected default color, got %q", tag.Color)
	}
}

func TestCreateTag_Duplicate(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)

	tagRepo.EXPECT().Create(gomock.Any()).Return(fmt.Errorf("UNIQUE constraint failed"))

	_, err := svc.CreateTag("Existing", "#FF4D4D")
	if err == nil {
		t.Fatal("expected error for duplicate tag")
	}
	// Now returns a conflict error
	if !apperrors.IsConflict(err) {
		t.Fatalf("expected conflict error, got: %v", err)
	}
}

func TestDeleteTag_Success(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)

	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "Test"}, nil)
	tagRepo.EXPECT().Delete(uint(1)).Return(nil)

	err := svc.DeleteTag(1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteTag_NotFound(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)

	tagRepo.EXPECT().GetByID(uint(99)).Return(nil, gorm.ErrRecordNotFound)

	err := svc.DeleteTag(99)
	if err == nil {
		t.Fatal("expected error for non-existent tag")
	}
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestGetVideoTags_Success(t *testing.T) {
	svc, tagRepo, videoRepo := newTestTagService(t)

	videoRepo.EXPECT().GetByID(uint(1)).Return(&data.Video{ID: 1}, nil)
	tagRepo.EXPECT().GetVideoTags(uint(1)).Return([]data.Tag{
		{ID: 1, Name: "Favorite", Color: "#FF4D4D"},
	}, nil)

	tags, err := svc.GetVideoTags(1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
}

func TestGetVideoTags_VideoNotFound(t *testing.T) {
	svc, _, videoRepo := newTestTagService(t)

	videoRepo.EXPECT().GetByID(uint(99)).Return(nil, gorm.ErrRecordNotFound)

	_, err := svc.GetVideoTags(99)
	if err == nil {
		t.Fatal("expected error for non-existent video")
	}
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestSetVideoTags_Success(t *testing.T) {
	svc, tagRepo, videoRepo := newTestTagService(t)

	videoRepo.EXPECT().GetByID(uint(1)).Return(&data.Video{ID: 1}, nil)
	tagRepo.EXPECT().SetVideoTags(uint(1), []uint{1, 2}).Return(nil)
	tagRepo.EXPECT().GetVideoTags(uint(1)).Return([]data.Tag{
		{ID: 1, Name: "Favorite", Color: "#FF4D4D"},
		{ID: 2, Name: "HD", Color: "#6366F1"},
	}, nil)

	tags, err := svc.SetVideoTags(1, []uint{1, 2})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
}

func TestSetVideoTags_VideoNotFound(t *testing.T) {
	svc, _, videoRepo := newTestTagService(t)

	videoRepo.EXPECT().GetByID(uint(99)).Return(nil, gorm.ErrRecordNotFound)

	_, err := svc.SetVideoTags(99, []uint{1})
	if err == nil {
		t.Fatal("expected error for non-existent video")
	}
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestSetVideoTags_EmptyTagIDs(t *testing.T) {
	svc, tagRepo, videoRepo := newTestTagService(t)

	videoRepo.EXPECT().GetByID(uint(1)).Return(&data.Video{ID: 1}, nil)
	tagRepo.EXPECT().SetVideoTags(uint(1), []uint{}).Return(nil)
	tagRepo.EXPECT().GetVideoTags(uint(1)).Return([]data.Tag{}, nil)

	tags, err := svc.SetVideoTags(1, []uint{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(tags))
	}
}

func TestCreateTag_ValidColorFormats(t *testing.T) {
	tests := []struct {
		name    string
		color   string
		wantErr bool
	}{
		{"uppercase hex", "#FF4D4D", false},
		{"lowercase hex", "#ff4d4d", false},
		{"mixed case hex", "#Ff4D4d", false},
		{"missing hash", "FF4D4D", true},
		{"too short", "#FFF", true},
		{"too long", "#FF4D4D4D", true},
		{"invalid chars", "#ZZZZZZ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, tagRepo, _ := newTestTagService(t)

			if !tt.wantErr {
				tagRepo.EXPECT().Create(gomock.Any()).Return(nil)
			}

			_, err := svc.CreateTag("Test", tt.color)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
