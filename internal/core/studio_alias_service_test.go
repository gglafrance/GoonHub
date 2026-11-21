package core

import (
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/lib/pq"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestStudioService(t *testing.T) (*StudioService, *mocks.MockStudioRepository, *mocks.MockSceneRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	studioRepo := mocks.NewMockStudioRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	return NewStudioService(studioRepo, sceneRepo, zap.NewNop()), studioRepo, sceneRepo
}

func TestStudioResolveAndGetByName(t *testing.T) {
	svc, repo, _ := newTestStudioService(t)
	repo.EXPECT().FindByNameOrAlias([]string{"bz"}, uint(0)).Return([]data.Studio{
		{ID: 4, Name: "Brazzers", Aliases: pq.StringArray{"BZ"}},
	}, nil).Times(2)

	res, err := svc.Resolve("bz")
	if err != nil {
		t.Fatal(err)
	}
	if res.Studio.ID != 4 || res.MatchedAlias != "BZ" || res.Candidates != 1 {
		t.Fatalf("got %+v", res)
	}
	studio, err := svc.GetByName("bz")
	if err != nil || studio.ID != 4 {
		t.Fatalf("GetByName by alias: %v %v", studio, err)
	}

	repo.EXPECT().FindByNameOrAlias([]string{"nope"}, uint(0)).Return(nil, nil)
	if _, err := svc.Resolve("nope"); !apperrors.IsNotFound(err) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestStudioGetOrCreateByNameUsesAliases(t *testing.T) {
	svc, repo, _ := newTestStudioService(t)
	repo.EXPECT().FindByNameOrAlias([]string{"BZ"}, uint(0)).Return([]data.Studio{{ID: 4, Name: "Brazzers", Aliases: pq.StringArray{"bz"}}}, nil)
	studio, err := svc.GetOrCreateByName("BZ")
	if err != nil || studio.ID != 4 {
		t.Fatalf("got %v %v, want the existing studio", studio, err)
	}

	repo.EXPECT().FindByNameOrAlias([]string{"New"}, uint(0)).Return(nil, nil)
	repo.EXPECT().Create(gomock.Any()).Return(nil)
	studio, err = svc.GetOrCreateByName("New")
	if err != nil || studio.Name != "New" {
		t.Fatalf("got %v %v, want a new studio", studio, err)
	}
}

func TestStudioCreateStrict(t *testing.T) {
	svc, repo, _ := newTestStudioService(t)
	repo.EXPECT().FindByNameOrAlias([]string{"Brazzers"}, uint(0)).Return([]data.Studio{{ID: 4, Name: "brazzers"}}, nil)
	_, err := svc.Create(CreateStudioInput{Name: "Brazzers", Strict: true})
	conflict, ok := apperrors.AsEntityConflict(err)
	if !ok || conflict.Field != "name" || conflict.Existing.(*data.Studio).ID != 4 {
		t.Fatalf("want a name conflict with studio 4, got %v", err)
	}
}

func TestStudioCreateAliases(t *testing.T) {
	svc, repo, _ := newTestStudioService(t)
	repo.EXPECT().FindByNameOrAlias([]string{"RK", "Used"}, uint(0)).Return([]data.Studio{{ID: 2, Name: "Used"}}, nil)
	repo.EXPECT().Create(gomock.Any()).Return(nil)
	studio, err := svc.Create(CreateStudioInput{Name: "Reality Kings", Aliases: []string{"RK", "rk", "Used", "reality kings"}})
	if err != nil {
		t.Fatal(err)
	}
	if !sameStrings(studio.Aliases, []string{"RK"}) {
		t.Fatalf("aliases = %#v, want [RK]", studio.Aliases)
	}
}

func TestStudioUpdateStrictAliasConflict(t *testing.T) {
	svc, repo, _ := newTestStudioService(t)
	repo.EXPECT().GetByID(uint(1)).Return(&data.Studio{ID: 1, Name: "Reality Kings"}, nil)
	repo.EXPECT().FindByNameOrAlias([]string{"Used"}, uint(1)).Return([]data.Studio{{ID: 2, Name: "Other", Aliases: pq.StringArray{"used"}}}, nil)
	aliases := []string{"Used"}
	_, err := svc.Update(1, UpdateStudioInput{Aliases: &aliases, Strict: true})
	if conflict, ok := apperrors.AsEntityConflict(err); !ok || conflict.Field != "aliases" {
		t.Fatalf("want an alias conflict, got %v", err)
	}
}

func TestStudioMerge(t *testing.T) {
	svc, repo, sceneRepo := newTestStudioService(t)
	idx := &bulkRecordingIndexer{}
	svc.SetIndexer(idx)
	target := &data.Studio{ID: 1, Name: "Brazzers", Logo: "/studio-logos/a.png"}
	source := &data.Studio{ID: 2, Name: "Brazzers Network", Aliases: pq.StringArray{"BZN"}, Logo: "/studio-logos/b.png", Poster: "/p.jpg"}
	repo.EXPECT().GetByID(uint(1)).Return(target, nil)
	repo.EXPECT().GetByID(uint(2)).Return(source, nil)
	repo.EXPECT().Merge(gomock.Any(), source).DoAndReturn(func(merged *data.Studio, _ *data.Studio) (*data.StudioMergeStats, []uint, error) {
		if merged.Logo != "/studio-logos/a.png" || merged.Poster != "/p.jpg" {
			t.Fatalf("logo must be kept and the missing poster taken: %+v", merged)
		}
		if !sameStrings(merged.Aliases, []string{"Brazzers Network", "BZN"}) {
			t.Fatalf("aliases = %#v", merged.Aliases)
		}
		return &data.StudioMergeStats{Scenes: 2, ChildStudios: 1}, []uint{8, 9}, nil
	})
	sceneRepo.EXPECT().GetByIDs([]uint{8, 9}).Return([]data.Scene{{ID: 8}, {ID: 9}}, nil)

	res, err := svc.Merge(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scenes != 2 || res.ChildStudios != 1 || res.LogoFromSource {
		t.Fatalf("unexpected result %+v", res)
	}
	if len(idx.bulk) != 2 {
		t.Fatalf("re-indexed %v, want the moved scenes", idx.bulk)
	}
	if _, err := svc.Merge(3, 3); !apperrors.IsValidation(err) {
		t.Fatalf("merging into itself: want a validation error, got %v", err)
	}
}

type fakeStudioResolver map[string]string

func (f fakeStudioResolver) Resolve(name string) (*StudioResolution, error) {
	if canonical, ok := f[name]; ok {
		return &StudioResolution{Studio: &data.Studio{Name: canonical}}, nil
	}
	return nil, apperrors.ErrStudioNotFoundByName(name)
}

func TestCanonicalStudioName(t *testing.T) {
	r := fakeStudioResolver{"BZ": "Brazzers"}
	cases := []struct {
		resolver StudioNameResolver
		in, want string
	}{
		{r, "BZ", "Brazzers"},
		{r, "Unknown", "Unknown"},
		{r, "", ""},
		{nil, "BZ", "BZ"},
	}
	for _, c := range cases {
		if got := canonicalStudioName(c.resolver, c.in); got != c.want {
			t.Fatalf("canonicalStudioName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
