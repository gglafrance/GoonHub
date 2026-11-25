package core

import (
	"strings"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newTestMarkerLabelService(t *testing.T) (*MarkerLabelService, *mocks.MockMarkerLabelRepository, *mocks.MockTagRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockMarkerLabelRepository(ctrl)
	tags := mocks.NewMockTagRepository(ctrl)
	return NewMarkerLabelService(repo, tags, zap.NewNop()), repo, tags
}

func strp(s string) *string { return &s }

func TestMarkerLabelService_CreateValidation(t *testing.T) {
	tests := []struct {
		name  string
		in    MarkerLabelInput
		field string
	}{
		{"no name", MarkerLabelInput{}, "name"},
		{"blank name", MarkerLabelInput{Name: strp("   ")}, "name"},
		{"long name", MarkerLabelInput{Name: strp(strings.Repeat("a", 101))}, "name"},
		{"bad color", MarkerLabelInput{Name: strp("Kiss"), Color: strp("red")}, "color"},
		{"long description", MarkerLabelInput{Name: strp("Kiss"), Description: strp(strings.Repeat("d", 501))}, "description"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newTestMarkerLabelService(t)
			_, err := svc.Create(tt.in)
			if !apperrors.IsValidation(err) || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("expected a validation error on %s, got %v", tt.field, err)
			}
		})
	}
}

func TestMarkerLabelService_Create(t *testing.T) {
	svc, repo, _ := newTestMarkerLabelService(t)
	repo.EXPECT().FindByName("Kiss").Return(nil, nil)
	repo.EXPECT().Create(gomock.Any()).DoAndReturn(func(l *data.MarkerLabel) error {
		if l.Name != "Kiss" || l.Color != "#FF4D4D" || l.Description != "a kiss" {
			t.Fatalf("unexpected label %+v", l)
		}
		l.ID = 3
		return nil
	})
	label, err := svc.Create(MarkerLabelInput{Name: strp("  Kiss "), Color: strp("#ff4d4d"), Description: strp(" a kiss ")})
	if err != nil || label.ID != 3 {
		t.Fatalf("create = %+v, %v", label, err)
	}
}

func TestMarkerLabelService_CreateConflict(t *testing.T) {
	svc, repo, _ := newTestMarkerLabelService(t)
	repo.EXPECT().FindByName("kiss").Return(&data.MarkerLabel{ID: 2, Name: "Kiss"}, nil)
	_, err := svc.Create(MarkerLabelInput{Name: strp("kiss")})
	conflict, ok := apperrors.AsEntityConflict(err)
	if !ok {
		t.Fatalf("expected an entity conflict, got %v", err)
	}
	if existing, _ := conflict.Existing.(data.MarkerLabel); existing.ID != 2 {
		t.Fatalf("conflict existing = %+v", conflict.Existing)
	}
}

func TestMarkerLabelService_Update(t *testing.T) {
	t.Run("rename", func(t *testing.T) {
		svc, repo, _ := newTestMarkerLabelService(t)
		repo.EXPECT().GetByID(uint(2)).Return(&data.MarkerLabel{ID: 2, Name: "Kiss", Color: "#FFFFFF"}, nil)
		repo.EXPECT().FindByName("Kissing").Return(nil, nil)
		repo.EXPECT().Update(gomock.Any(), "Kiss").DoAndReturn(func(l *data.MarkerLabel, _ string) (int64, error) {
			if l.Name != "Kissing" || l.Color != "#FFFFFF" {
				t.Fatalf("unexpected label %+v", l)
			}
			return 1, nil
		})
		if _, err := svc.Update(2, MarkerLabelInput{Name: strp("Kissing")}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("case of its own name", func(t *testing.T) {
		svc, repo, _ := newTestMarkerLabelService(t)
		repo.EXPECT().GetByID(uint(2)).Return(&data.MarkerLabel{ID: 2, Name: "kiss"}, nil)
		repo.EXPECT().FindByName("Kiss").Return(&data.MarkerLabel{ID: 2, Name: "kiss"}, nil)
		repo.EXPECT().Update(gomock.Any(), "kiss").Return(int64(0), nil)
		label, err := svc.Update(2, MarkerLabelInput{Name: strp("Kiss")})
		if err != nil || label.Name != "Kiss" {
			t.Fatalf("update = %+v, %v", label, err)
		}
	})
	t.Run("name of another label", func(t *testing.T) {
		svc, repo, _ := newTestMarkerLabelService(t)
		repo.EXPECT().GetByID(uint(2)).Return(&data.MarkerLabel{ID: 2, Name: "Kiss"}, nil)
		repo.EXPECT().FindByName("hug").Return(&data.MarkerLabel{ID: 5, Name: "Hug"}, nil)
		_, err := svc.Update(2, MarkerLabelInput{Name: strp("hug")})
		if _, ok := apperrors.AsEntityConflict(err); !ok {
			t.Fatalf("expected an entity conflict, got %v", err)
		}
	})
	t.Run("unknown label", func(t *testing.T) {
		svc, repo, _ := newTestMarkerLabelService(t)
		repo.EXPECT().GetByID(uint(9)).Return(nil, gorm.ErrRecordNotFound)
		if _, err := svc.Update(9, MarkerLabelInput{Color: strp("#000000")}); !apperrors.IsNotFound(err) {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("nothing to change", func(t *testing.T) {
		svc, _, _ := newTestMarkerLabelService(t)
		if _, err := svc.Update(2, MarkerLabelInput{}); !apperrors.IsValidation(err) {
			t.Fatalf("expected a validation error, got %v", err)
		}
	})
}

func TestMarkerLabelService_Merge(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		svc, _, _ := newTestMarkerLabelService(t)
		if _, err := svc.Merge(0, []uint{1}, true); !apperrors.IsValidation(err) {
			t.Fatalf("no target: %v", err)
		}
		if _, err := svc.Merge(1, []uint{1}, true); !apperrors.IsValidation(err) {
			t.Fatalf("target in sources: %v", err)
		}
		if _, err := svc.Merge(1, []uint{0}, true); !apperrors.IsValidation(err) {
			t.Fatalf("no source: %v", err)
		}
	})
	t.Run("missing source", func(t *testing.T) {
		svc, repo, _ := newTestMarkerLabelService(t)
		repo.EXPECT().GetByID(uint(1)).Return(&data.MarkerLabel{ID: 1, Name: "Kiss"}, nil)
		repo.EXPECT().GetByIDs([]uint{2, 3}).Return([]data.MarkerLabel{{ID: 2, Name: "kiss2"}}, nil)
		if _, err := svc.Merge(1, []uint{2, 3, 2}, false); !apperrors.IsNotFound(err) {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("merge", func(t *testing.T) {
		svc, repo, _ := newTestMarkerLabelService(t)
		target := data.MarkerLabel{ID: 1, Name: "Kiss"}
		sources := []data.MarkerLabel{{ID: 2, Name: "Smooch"}}
		repo.EXPECT().GetByID(uint(1)).Return(&target, nil)
		repo.EXPECT().GetByIDs([]uint{2}).Return(sources, nil)
		repo.EXPECT().Merge(target, sources, true).Return(&data.MarkerLabelMergeStats{MarkersMoved: 4, SourcesDeleted: 1}, nil)
		res, err := svc.Merge(1, []uint{2}, true)
		if err != nil || res.MarkersMoved != 4 || !res.DryRun || res.Target.ID != 1 {
			t.Fatalf("merge = %+v, %v", res, err)
		}
	})
}

func TestMarkerLabelService_DeleteAndDefaultTags(t *testing.T) {
	svc, repo, tags := newTestMarkerLabelService(t)
	repo.EXPECT().GetByIDs([]uint{4, 5}).Return([]data.MarkerLabel{{ID: 5}, {ID: 4}}, nil)
	repo.EXPECT().Delete([]uint{4, 5}).Return(int64(7), nil)
	res, err := svc.Delete([]uint{4, 5, 4, 0})
	if err != nil || res.Deleted != 2 || res.MarkersCleared != 7 {
		t.Fatalf("delete = %+v, %v", res, err)
	}
	if _, err := svc.Delete(nil); !apperrors.IsValidation(err) {
		t.Fatalf("delete nothing: %v", err)
	}

	repo.EXPECT().GetByID(uint(4)).Return(&data.MarkerLabel{ID: 4}, nil).Times(3)
	tags.EXPECT().GetByIDs([]uint{8, 9}).Return([]data.Tag{{ID: 8}}, nil)
	if _, err := svc.SetDefaultTags(4, []uint{8, 9}); !apperrors.IsValidation(err) {
		t.Fatalf("unknown tag: %v", err)
	}
	tags.EXPECT().GetByIDs([]uint{8}).Return([]data.Tag{{ID: 8}}, nil)
	repo.EXPECT().SetDefaultTags(uint(4), []uint{8}).Return(nil)
	repo.EXPECT().GetDefaultTags(uint(4)).Return([]data.Tag{{ID: 8}}, nil)
	got, err := svc.SetDefaultTags(4, []uint{8, 8})
	if err != nil || len(got) != 1 {
		t.Fatalf("set default tags = %+v, %v", got, err)
	}
}

func TestMarkerService_CreateMarkerLabel(t *testing.T) {
	setup := func(t *testing.T) (*MarkerService, *mocks.MockMarkerRepository, *mocks.MockMarkerLabelRepository) {
		ctrl := gomock.NewController(t)
		markerRepo := mocks.NewMockMarkerRepository(ctrl)
		labelRepo := mocks.NewMockMarkerLabelRepository(ctrl)
		sceneRepo := mocks.NewMockSceneRepository(ctrl)
		sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2, Duration: 100}, nil)
		markerRepo.EXPECT().CountByUserAndScene(uint(1), uint(2)).Return(int64(0), nil)
		svc := &MarkerService{markerRepo: markerRepo, labelRepo: labelRepo, sceneRepo: sceneRepo, logger: zap.NewNop()}
		// thumbnails are made inline without workers: the scene has no file, it is skipped
		return svc, markerRepo, labelRepo
	}

	t.Run("existing label, any case", func(t *testing.T) {
		svc, markerRepo, labelRepo := setup(t)
		labelRepo.EXPECT().FindByName("kiss").Return(&data.MarkerLabel{ID: 6, Name: "Kiss", Color: "#FF0000"}, nil)
		markerRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(m *data.UserSceneMarker) error {
			m.ID = 11
			return nil
		})
		markerRepo.EXPECT().ApplyLabelTagsToMarker(uint(11)).Return(nil)
		marker, err := svc.CreateMarker(1, 2, 10, nil, " kiss ", "", false)
		if err != nil {
			t.Fatal(err)
		}
		if marker.LabelID == nil || *marker.LabelID != 6 || marker.Label != "Kiss" || marker.Color != "#FF0000" {
			t.Fatalf("marker = %+v", marker)
		}
	})
	t.Run("unknown label without the permission", func(t *testing.T) {
		svc, _, labelRepo := setup(t)
		labelRepo.EXPECT().FindByName("New").Return(nil, nil)
		_, err := svc.CreateMarker(1, 2, 10, nil, "New", "", false)
		if !apperrors.IsValidation(err) || !strings.Contains(err.Error(), "unknown marker label") {
			t.Fatalf("expected a validation error, got %v", err)
		}
	})
	t.Run("unknown label with the permission", func(t *testing.T) {
		svc, markerRepo, labelRepo := setup(t)
		labelRepo.EXPECT().FindByName("New").Return(nil, nil)
		labelRepo.EXPECT().Ensure("New").Return(&data.MarkerLabel{ID: 7, Name: "New", Color: "#FFFFFF"}, true, nil)
		markerRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(m *data.UserSceneMarker) error {
			m.ID = 12
			return nil
		})
		markerRepo.EXPECT().ApplyLabelTagsToMarker(uint(12)).Return(nil)
		marker, err := svc.CreateMarker(1, 2, 10, nil, "New", "#00FF00", true)
		if err != nil || *marker.LabelID != 7 || marker.Color != "#00FF00" {
			t.Fatalf("marker = %+v, %v", marker, err)
		}
	})
	t.Run("no label", func(t *testing.T) {
		svc, markerRepo, _ := setup(t)
		markerRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(m *data.UserSceneMarker) error {
			if m.LabelID != nil || m.Label != "" || m.Color != "#FFFFFF" {
				t.Fatalf("marker = %+v", m)
			}
			m.ID = 13
			return nil
		})
		if _, err := svc.CreateMarker(1, 2, 10, nil, "  ", "", false); err != nil {
			t.Fatal(err)
		}
	})
}

func TestMarkerService_UpdateMarkerLabel(t *testing.T) {
	ctrl := gomock.NewController(t)
	markerRepo := mocks.NewMockMarkerRepository(ctrl)
	labelRepo := mocks.NewMockMarkerLabelRepository(ctrl)
	svc := &MarkerService{markerRepo: markerRepo, labelRepo: labelRepo, logger: zap.NewNop()}
	old := uint(6)

	t.Run("new label swaps the label tags", func(t *testing.T) {
		marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, LabelID: &old, Label: "Kiss"}
		markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)
		labelRepo.EXPECT().FindByName("Hug").Return(&data.MarkerLabel{ID: 8, Name: "Hug"}, nil)
		markerRepo.EXPECT().Update(marker).Return(nil)
		markerRepo.EXPECT().ApplyLabelTagsToMarker(uint(3)).Return(nil)
		got, err := svc.UpdateMarker(1, 3, strp("Hug"), nil, nil, nil, false, false)
		if err != nil || *got.LabelID != 8 || got.Label != "Hug" {
			t.Fatalf("update = %+v, %v", got, err)
		}
	})
	t.Run("same label keeps the tags", func(t *testing.T) {
		marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, LabelID: &old, Label: "Kiss"}
		markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)
		labelRepo.EXPECT().FindByName("KISS").Return(&data.MarkerLabel{ID: 6, Name: "Kiss"}, nil)
		markerRepo.EXPECT().Update(marker).Return(nil)
		if _, err := svc.UpdateMarker(1, 3, strp("KISS"), nil, nil, nil, false, false); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("cleared label", func(t *testing.T) {
		marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, LabelID: &old, Label: "Kiss"}
		markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)
		markerRepo.EXPECT().Update(marker).Return(nil)
		markerRepo.EXPECT().ApplyLabelTagsToMarker(uint(3)).Return(nil)
		got, err := svc.UpdateMarker(1, 3, strp(""), nil, nil, nil, false, false)
		if err != nil || got.LabelID != nil || got.Label != "" {
			t.Fatalf("update = %+v, %v", got, err)
		}
	})
	t.Run("unknown label refused", func(t *testing.T) {
		marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, LabelID: &old, Label: "Kiss"}
		markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)
		labelRepo.EXPECT().FindByName("Nope").Return(nil, nil)
		if _, err := svc.UpdateMarker(1, 3, strp("Nope"), nil, nil, nil, false, false); !apperrors.IsValidation(err) {
			t.Fatalf("expected a validation error, got %v", err)
		}
	})
}
