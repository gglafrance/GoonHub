package core

import (
	"strings"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func TestNormalizeSourceURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"empty clears", "", "", false},
		{"blank clears", "   ", "", false},
		{"https", "https://www.lpsg.com/threads/x.1/post-2", "https://www.lpsg.com/threads/x.1/post-2", false},
		{"http with query", "http://example.com/watch?v=1&t=2", "http://example.com/watch?v=1&t=2", false},
		{"trimmed", "  https://example.com/a  ", "https://example.com/a", false},
		{"no scheme", "example.com/a", "", true},
		{"other scheme", "ftp://example.com/a", "", true},
		{"javascript", "javascript:alert(1)", "", true},
		{"no host", "https:///path", "", true},
		{"inner space", "https://example.com/a b", "", true},
		{"two urls", "https://a.com https://b.com", "", true},
		{"too long", "https://example.com/" + strings.Repeat("a", MaxSourceURLLen), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeSourceURL(tt.in)
			if tt.wantErr {
				if !apperrors.IsValidation(err) {
					t.Fatalf("expected a validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpdateSceneSourceURL_Success(t *testing.T) {
	svc, sceneRepo := newTestSceneService(t)

	gomock.InOrder(
		sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1}, nil),
		sceneRepo.EXPECT().UpdateSourceURL(uint(1), "https://example.com/v/1").Return(nil),
		sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1, SourceURL: "https://example.com/v/1"}, nil),
	)

	scene, err := svc.UpdateSceneSourceURL(1, " https://example.com/v/1 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scene.SourceURL != "https://example.com/v/1" {
		t.Fatalf("expected the new source, got %q", scene.SourceURL)
	}
}

func TestUpdateSceneSourceURL_Clear(t *testing.T) {
	svc, sceneRepo := newTestSceneService(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1, SourceURL: "https://example.com"}, nil).Times(2)
	sceneRepo.EXPECT().UpdateSourceURL(uint(1), "").Return(nil)

	if _, err := svc.UpdateSceneSourceURL(1, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateSceneSourceURL_Invalid(t *testing.T) {
	// No repository expectation: an invalid URL never reaches the database.
	svc, _ := newTestSceneService(t)

	_, err := svc.UpdateSceneSourceURL(1, "not a url")
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error, got %v", err)
	}
}

func TestUpdateSceneSourceURL_NotFound(t *testing.T) {
	svc, sceneRepo := newTestSceneService(t)

	sceneRepo.EXPECT().GetByID(uint(99)).Return(nil, gorm.ErrRecordNotFound)

	_, err := svc.UpdateSceneSourceURL(99, "https://example.com")
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}

func TestImportScene_SourceURL(t *testing.T) {
	svc, _ := newTestImportService(t)

	in := baseSceneInput()
	in.SourceURL = ptr("  https://www.xvideos.com/video.abc/x  ")
	p, err := svc.prepareScene(1, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := svc.buildNewScene(in, p).SourceURL; got != "https://www.xvideos.com/video.abc/x" {
		t.Fatalf("new scene: got source %q", got)
	}
	if got := svc.buildUpdateFields(in, p)["source_url"]; got != "https://www.xvideos.com/video.abc/x" {
		t.Fatalf("update: got source %v", got)
	}

	// absent: an update leaves the source alone
	in.SourceURL = nil
	if p, err = svc.prepareScene(1, in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := svc.buildUpdateFields(in, p)["source_url"]; ok {
		t.Fatal("an absent source_url must not be updated")
	}

	// invalid: rejected before anything is written
	in.SourceURL = ptr("javascript:alert(1)")
	if _, err := svc.ImportScene(1, in); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error, got %v", err)
	}
}

func TestMergeMetadata_SourceURL(t *testing.T) {
	svc, _, sceneRepo, tagRepo, actorRepo, markerRepo, interactionRepo := newTestDuplicateService(t)

	tagRepo.EXPECT().GetSceneTags(gomock.Any()).Return([]data.Tag{}, nil).AnyTimes()
	actorRepo.EXPECT().GetSceneActors(gomock.Any()).Return([]data.Actor{}, nil).AnyTimes()
	studioID := uint(5)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed(gomock.Any()).Return([]data.Scene{
		{ID: 10, Title: "Best", StudioID: &studioID},
		{ID: 20, Title: "No source"},
		{ID: 30, Title: "Has source", SourceURL: "https://example.com/v/30"},
	}, nil)
	// the best scene has no source: the first other scene with one gives it
	sceneRepo.EXPECT().UpdateSourceURL(uint(10), "https://example.com/v/30").Return(nil)
	markerRepo.EXPECT().ReassignMarkersToScene(gomock.Any(), uint(10)).Return(nil).Times(2)
	interactionRepo.EXPECT().ReassignInteractionsToScene(gomock.Any(), uint(10)).Return(nil).Times(2)

	if err := svc.mergeMetadata(10, []uint{20, 30}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeMetadata_KeepsBestSourceURL(t *testing.T) {
	svc, _, sceneRepo, tagRepo, actorRepo, markerRepo, interactionRepo := newTestDuplicateService(t)

	tagRepo.EXPECT().GetSceneTags(gomock.Any()).Return([]data.Tag{}, nil).AnyTimes()
	actorRepo.EXPECT().GetSceneActors(gomock.Any()).Return([]data.Actor{}, nil).AnyTimes()
	studioID := uint(5)
	// no UpdateSourceURL expectation: the best scene's own source stays
	sceneRepo.EXPECT().GetByIDsIncludingTrashed(gomock.Any()).Return([]data.Scene{
		{ID: 10, Title: "Best", StudioID: &studioID, SourceURL: "https://example.com/v/10"},
		{ID: 20, Title: "Other", SourceURL: "https://example.com/v/20"},
	}, nil)
	markerRepo.EXPECT().ReassignMarkersToScene(uint(20), uint(10)).Return(nil)
	interactionRepo.EXPECT().ReassignInteractionsToScene(uint(20), uint(10)).Return(nil)

	if err := svc.mergeMetadata(10, []uint{20}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
