package core

import (
	"errors"
	"strings"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"gorm.io/gorm"
)

func TestUpdateTag_Validation(t *testing.T) {
	svc, _, _ := newTestTagService(t)
	cases := []struct {
		name string
		upd  TagUpdate
	}{
		{"nothing to change", TagUpdate{}},
		{"empty name", TagUpdate{Name: ptr("")}},
		{"blank name", TagUpdate{Name: ptr("   ")}},
		{"name too long", TagUpdate{Name: ptr(strings.Repeat("a", 101))}},
		{"bad color with a name", TagUpdate{Name: ptr("Anal"), Color: ptr("red")}},
	}
	for _, c := range cases {
		if _, err := svc.UpdateTag(1, c.upd); !apperrors.IsValidation(err) {
			t.Errorf("%s: want a validation error, got %v", c.name, err)
		}
	}
}

func TestUpdateTag_RenameNotFound(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)
	tagRepo.EXPECT().GetByID(uint(9)).Return(nil, gorm.ErrRecordNotFound)
	if _, err := svc.UpdateTag(9, TagUpdate{Name: ptr("Anal")}); !apperrors.IsNotFound(err) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestUpdateTag_RenameConflicts(t *testing.T) {
	all := []data.Tag{
		{ID: 1, Name: "cumshot"},
		{ID: 2, Name: "Anal"},
		{ID: 3, Name: "Big Tits"},
	}
	cases := []struct {
		name     string
		input    string
		existing uint
	}{
		{"exact name", "Anal", 2},
		{"other case", "anal", 2},
		{"extra spaces", "  big   tits ", 3},
	}
	for _, c := range cases {
		svc, tagRepo, _ := newTestTagService(t)
		tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "cumshot", Color: "#6B7280"}, nil)
		tagRepo.EXPECT().List().Return(all, nil)
		_, err := svc.UpdateTag(1, TagUpdate{Name: ptr(c.input)})
		conflict, ok := apperrors.AsEntityConflict(err)
		if !ok || !apperrors.IsConflict(err) {
			t.Fatalf("%s: want an entity conflict, got %v", c.name, err)
		}
		existing, _ := conflict.Existing.(data.Tag)
		if existing.ID != c.existing || conflict.Field != "name" || !strings.Contains(err.Error(), "merge into it") {
			t.Fatalf("%s: conflict = %+v (%v)", c.name, conflict, err)
		}
	}
}

func TestUpdateTag_ConflictPrefersExactName(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)
	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "x"}, nil)
	tagRepo.EXPECT().List().Return([]data.Tag{{ID: 2, Name: "anal"}, {ID: 3, Name: "Anal"}, {ID: 1, Name: "x"}}, nil)
	_, err := svc.UpdateTag(1, TagUpdate{Name: ptr("Anal")})
	conflict, ok := apperrors.AsEntityConflict(err)
	if !ok || conflict.Existing.(data.Tag).ID != 3 {
		t.Fatalf("want the exact-name tag 3, got %v", err)
	}
}

func TestUpdateTag_RenameOwnCaseReindexes(t *testing.T) {
	svc, tagRepo, sceneRepo := newTestTagService(t)
	indexer := &bulkRecordingIndexer{}
	svc.SetIndexer(indexer)

	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "cumshot", Color: "#6B7280"}, nil)
	// the tag itself matches the key: not a conflict
	tagRepo.EXPECT().List().Return([]data.Tag{{ID: 1, Name: "cumshot"}, {ID: 2, Name: "Anal"}}, nil)
	tagRepo.EXPECT().RenameTag(data.Tag{ID: 1, Name: "Cumshot", Color: "#FF4D4D"}, "cumshot").Return(int64(2), []uint{10, 11}, nil)
	sceneRepo.EXPECT().GetByIDs([]uint{10, 11}).Return([]data.Scene{{ID: 10}, {ID: 11}}, nil)

	tag, err := svc.UpdateTag(1, TagUpdate{Name: ptr(" Cumshot "), Color: ptr("#ff4d4d")})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if tag.Name != "Cumshot" || tag.Color != "#FF4D4D" {
		t.Fatalf("tag = %+v", tag)
	}
	if len(indexer.bulk) != 2 || indexer.bulk[0] != 10 || indexer.bulk[1] != 11 {
		t.Fatalf("re-indexed scenes = %v", indexer.bulk)
	}
}

func TestUpdateTag_SameNameIsAColorUpdate(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)
	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "Anal", Color: "#6B7280"}, nil)
	tagRepo.EXPECT().Update(&data.Tag{ID: 1, Name: "Anal", Color: "#6B7280"}).Return(nil)
	if _, err := svc.UpdateTag(1, TagUpdate{Name: ptr(" Anal ")}); err != nil {
		t.Fatalf("unchanged name: %v", err)
	}
}

func TestUpdateTag_RenameRace(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)
	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "a"}, nil)
	tagRepo.EXPECT().List().Return([]data.Tag{{ID: 1, Name: "a"}}, nil)
	tagRepo.EXPECT().RenameTag(data.Tag{ID: 1, Name: "b"}, "a").Return(int64(0), nil, data.ErrTagNameTaken)
	// the second check still does not see the tag that took the name
	tagRepo.EXPECT().List().Return([]data.Tag{{ID: 1, Name: "a"}}, nil)
	if _, err := svc.UpdateTag(1, TagUpdate{Name: ptr("b")}); !apperrors.IsConflict(err) {
		t.Fatalf("want a conflict, got %v", err)
	}

	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "a"}, nil)
	tagRepo.EXPECT().List().Return([]data.Tag{{ID: 1, Name: "a"}}, nil)
	tagRepo.EXPECT().RenameTag(data.Tag{ID: 1, Name: "b"}, "a").Return(int64(0), nil, errors.New("boom"))
	if _, err := svc.UpdateTag(1, TagUpdate{Name: ptr("b")}); err == nil || apperrors.IsConflict(err) {
		t.Fatalf("want an internal error, got %v", err)
	}
}
