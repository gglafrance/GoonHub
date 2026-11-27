package core

import (
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func TestNormalizeTagName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Cumshot", "cumshot"},
		{"  cumshot ", "cumshot"},
		{"Big   Tits", "big tits"},
		{"big\ttits", "big tits"},
		{"ÉCOLE", "école"},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := NormalizeTagName(c.in); got != c.want {
			t.Errorf("NormalizeTagName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGroupDuplicateTags(t *testing.T) {
	tags := []data.TagWithCount{
		{Tag: data.Tag{ID: 1, Name: "Cumshot"}, SceneCount: 3},
		{Tag: data.Tag{ID: 2, Name: "cumshot"}, SceneCount: 10},
		{Tag: data.Tag{ID: 3, Name: " Cumshot  "}, SceneCount: 10},
		{Tag: data.Tag{ID: 4, Name: "Anal"}, SceneCount: 5},
		{Tag: data.Tag{ID: 5, Name: "Big  Tits"}, SceneCount: 0},
		{Tag: data.Tag{ID: 6, Name: "big tits"}, SceneCount: 0},
	}
	groups := GroupDuplicateTags(tags)
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(groups), groups)
	}
	if groups[0].Key != "big tits" || groups[1].Key != "cumshot" {
		t.Fatalf("groups not sorted by key: %q, %q", groups[0].Key, groups[1].Key)
	}
	if groups[0].SuggestedTargetID != 5 {
		t.Fatalf("tie on scene count must keep the oldest tag, got %d", groups[0].SuggestedTargetID)
	}
	cum := groups[1]
	if cum.SuggestedTargetID != 2 || len(cum.Tags) != 3 || cum.Tags[0].ID != 2 || cum.Tags[1].ID != 3 || cum.Tags[2].ID != 1 {
		t.Fatalf("cumshot group = %+v", cum)
	}
	if got := GroupDuplicateTags(nil); got == nil || len(got) != 0 {
		t.Fatalf("no tags must give an empty (non-nil) list, got %#v", got)
	}
}

func TestFindDuplicates(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)
	tagRepo.EXPECT().ListWithCounts().Return([]data.TagWithCount{
		{Tag: data.Tag{ID: 1, Name: "Cumshot"}, SceneCount: 3},
		{Tag: data.Tag{ID: 2, Name: "cumshot"}, SceneCount: 1},
	}, nil)
	groups, err := svc.FindDuplicates()
	if err != nil || len(groups) != 1 || groups[0].SuggestedTargetID != 1 {
		t.Fatalf("groups = %+v, err = %v", groups, err)
	}
}

func TestMergeTags_Validation(t *testing.T) {
	svc, _, _ := newTestTagService(t)
	cases := []struct {
		name    string
		target  uint
		sources []uint
	}{
		{"no target", 0, []uint{2}},
		{"no sources", 1, nil},
		{"only zero sources", 1, []uint{0}},
		{"target among sources", 1, []uint{2, 1}},
	}
	for _, c := range cases {
		if _, err := svc.MergeTags(c.target, c.sources, false); !apperrors.IsValidation(err) {
			t.Errorf("%s: want a validation error, got %v", c.name, err)
		}
	}
}

func TestMergeTags_NotFound(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)
	tagRepo.EXPECT().GetByID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
	if _, err := svc.MergeTags(1, []uint{2}, false); !apperrors.IsNotFound(err) {
		t.Fatalf("missing target: want not found, got %v", err)
	}

	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "Cumshot"}, nil)
	tagRepo.EXPECT().GetByIDs([]uint{2, 3}).Return([]data.Tag{{ID: 2, Name: "cumshot"}}, nil)
	if _, err := svc.MergeTags(1, []uint{2, 3}, false); !apperrors.IsNotFound(err) {
		t.Fatalf("missing source: want not found, got %v", err)
	}
}

func TestMergeTags_SuccessReindexes(t *testing.T) {
	svc, tagRepo, sceneRepo := newTestTagService(t)
	indexer := &bulkRecordingIndexer{}
	svc.SetIndexer(indexer)

	target := data.Tag{ID: 1, Name: "Cumshot"}
	sources := []data.Tag{{ID: 3, Name: "cumshot "}, {ID: 2, Name: "cumshot"}}
	tagRepo.EXPECT().GetByID(uint(1)).Return(&target, nil)
	tagRepo.EXPECT().GetByIDs([]uint{3, 2}).Return(sources, nil)
	tagRepo.EXPECT().MergeTags(target, gomock.Any(), false).DoAndReturn(
		func(_ data.Tag, got []data.Tag, _ bool) (*data.TagMergeStats, []uint, error) {
			if len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
				t.Fatalf("sources passed to the repository = %+v", got)
			}
			return &data.TagMergeStats{ScenesRetagged: 2, MarkersUpdated: 1, ReferencesUpdated: 1, SourcesDeleted: 2}, []uint{10, 11}, nil
		})
	sceneRepo.EXPECT().GetByIDs([]uint{10, 11}).Return([]data.Scene{{ID: 10}, {ID: 11}}, nil)

	// duplicates in the request are ignored
	res, err := svc.MergeTags(1, []uint{3, 2, 3}, false)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if res.Target.ID != 1 || len(res.Sources) != 2 || res.ScenesRetagged != 2 || res.SourcesDeleted != 2 || res.DryRun {
		t.Fatalf("result = %+v", res)
	}
	if len(indexer.bulk) != 2 || indexer.bulk[0] != 10 || indexer.bulk[1] != 11 {
		t.Fatalf("re-indexed scenes = %v", indexer.bulk)
	}
}

func TestMergeTags_DryRunDoesNotReindex(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)
	indexer := &bulkRecordingIndexer{}
	svc.SetIndexer(indexer)

	target := data.Tag{ID: 1, Name: "Cumshot"}
	tagRepo.EXPECT().GetByID(uint(1)).Return(&target, nil)
	tagRepo.EXPECT().GetByIDs([]uint{2}).Return([]data.Tag{{ID: 2, Name: "cumshot"}}, nil)
	tagRepo.EXPECT().MergeTags(target, gomock.Any(), true).Return(&data.TagMergeStats{ScenesRetagged: 4, SourcesDeleted: 1}, nil, nil)

	res, err := svc.MergeTags(1, []uint{2}, true)
	if err != nil || !res.DryRun || res.ScenesRetagged != 4 {
		t.Fatalf("dry run = %+v, %v", res, err)
	}
	if len(indexer.bulk) != 0 {
		t.Fatalf("dry run must not re-index, got %v", indexer.bulk)
	}
}

func TestUpdateTagColor(t *testing.T) {
	svc, tagRepo, _ := newTestTagService(t)

	for _, bad := range []string{"", "red", "#FFF", "#GGGGGG", "FF4D4D", "#FF4D4D0"} {
		if _, err := svc.UpdateTag(1, TagUpdate{Color: ptr(bad)}); !apperrors.IsValidation(err) {
			t.Errorf("color %q: want a validation error, got %v", bad, err)
		}
	}

	tagRepo.EXPECT().GetByID(uint(9)).Return(nil, gorm.ErrRecordNotFound)
	if _, err := svc.UpdateTag(9, TagUpdate{Color: ptr("#FF4D4D")}); !apperrors.IsNotFound(err) {
		t.Fatalf("unknown tag: want not found, got %v", err)
	}

	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "Anal", Color: "#6B7280"}, nil)
	tagRepo.EXPECT().Update(gomock.Any()).DoAndReturn(func(tag *data.Tag) error {
		if tag.ID != 1 || tag.Name != "Anal" || tag.Color != "#FF4D4D" {
			t.Fatalf("saved tag = %+v", tag)
		}
		return nil
	})
	tag, err := svc.UpdateTag(1, TagUpdate{Color: ptr("#ff4d4d")})
	if err != nil || tag.Color != "#FF4D4D" {
		t.Fatalf("updated tag = %+v, %v", tag, err)
	}
}
