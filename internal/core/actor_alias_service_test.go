package core

import (
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/lib/pq"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type bulkRecordingIndexer struct {
	recordingIndexer
	bulk []uint
}

func (r *bulkRecordingIndexer) BulkUpdateSceneIndex(scenes []data.Scene) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range scenes {
		r.bulk = append(r.bulk, s.ID)
	}
	return nil
}

func newTestActorService(t *testing.T) (*ActorService, *mocks.MockActorRepository, *mocks.MockSceneRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	actorRepo := mocks.NewMockActorRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	return NewActorService(actorRepo, sceneRepo, zap.NewNop()), actorRepo, sceneRepo
}

func TestActorResolve(t *testing.T) {
	svc, repo, _ := newTestActorService(t)

	// an alias match of an older actor, and a name match: the name wins
	repo.EXPECT().FindByNameOrAlias([]string{"Jane Doe"}, uint(0)).Return([]data.Actor{
		{ID: 1, Name: "Janie", Aliases: pq.StringArray{"jane doe"}},
		{ID: 2, Name: "Jane Doe"},
	}, nil)
	res, err := svc.Resolve("  Jane Doe ")
	if err != nil {
		t.Fatal(err)
	}
	if res.Actor.ID != 2 || res.MatchedAlias != "" || res.Candidates != 2 {
		t.Fatalf("got actor %d alias %q candidates %d", res.Actor.ID, res.MatchedAlias, res.Candidates)
	}

	repo.EXPECT().FindByNameOrAlias([]string{"jd"}, uint(0)).Return([]data.Actor{
		{ID: 3, Name: "Jane Doe", Aliases: pq.StringArray{"JD"}},
	}, nil)
	res, err = svc.Resolve("jd")
	if err != nil {
		t.Fatal(err)
	}
	if res.Actor.ID != 3 || res.MatchedAlias != "JD" {
		t.Fatalf("alias resolution: actor %d alias %q", res.Actor.ID, res.MatchedAlias)
	}

	repo.EXPECT().FindByNameOrAlias([]string{"nobody"}, uint(0)).Return([]data.Actor{}, nil)
	if _, err := svc.Resolve("nobody"); !apperrors.IsNotFound(err) {
		t.Fatalf("want not found, got %v", err)
	}

	if _, err := svc.Resolve("  "); !apperrors.IsValidation(err) {
		t.Fatalf("want a validation error for an empty name, got %v", err)
	}
}

func TestActorCreateStrictNameConflict(t *testing.T) {
	svc, repo, _ := newTestActorService(t)
	existing := data.Actor{ID: 7, Name: "Jane Doe", Aliases: pq.StringArray{"JD"}}
	repo.EXPECT().FindByNameOrAlias([]string{"jd"}, uint(0)).Return([]data.Actor{existing}, nil)

	_, err := svc.Create(CreateActorInput{Name: "jd", Strict: true})
	conflict, ok := apperrors.AsEntityConflict(err)
	if !ok {
		t.Fatalf("want an entity conflict, got %v", err)
	}
	if !apperrors.IsConflict(err) || apperrors.GetHTTPStatus(err) != 409 {
		t.Fatalf("conflict must be a 409 ConflictError: %v", err)
	}
	if conflict.Field != "name" || conflict.Existing.(*data.Actor).ID != 7 {
		t.Fatalf("got field %q existing %+v", conflict.Field, conflict.Existing)
	}
}

func TestActorCreateLenientAllowsDuplicateNameAndDropsUsedAliases(t *testing.T) {
	svc, repo, _ := newTestActorService(t)
	// no name check without strict; aliases checked once
	repo.EXPECT().FindByNameOrAlias([]string{"JD", "Janie"}, uint(0)).Return([]data.Actor{
		{ID: 7, Name: "Someone", Aliases: pq.StringArray{"janie"}},
	}, nil)
	var created *data.Actor
	repo.EXPECT().Create(gomock.Any()).DoAndReturn(func(a *data.Actor) error {
		created = a
		return nil
	})

	actor, err := svc.Create(CreateActorInput{Name: "Jane Doe", Aliases: []string{" JD ", "Janie", "jane doe"}})
	if err != nil {
		t.Fatal(err)
	}
	if created != actor || !sameStrings(actor.Aliases, []string{"JD"}) {
		t.Fatalf("aliases = %#v, want [JD]", actor.Aliases)
	}
}

func TestActorCreateStrictAliasConflict(t *testing.T) {
	svc, repo, _ := newTestActorService(t)
	repo.EXPECT().FindByNameOrAlias([]string{"Jane Doe"}, uint(0)).Return([]data.Actor{}, nil)
	repo.EXPECT().FindByNameOrAlias([]string{"Janie"}, uint(0)).Return([]data.Actor{{ID: 9, Name: "JANIE"}}, nil)

	_, err := svc.Create(CreateActorInput{Name: "Jane Doe", Aliases: []string{"Janie"}, Strict: true})
	conflict, ok := apperrors.AsEntityConflict(err)
	if !ok || conflict.Field != "aliases" || conflict.Value != "Janie" {
		t.Fatalf("want an alias conflict on Janie, got %v", err)
	}
}

func TestActorUpdateStrictRename(t *testing.T) {
	svc, repo, _ := newTestActorService(t)
	repo.EXPECT().GetByID(uint(1)).Return(&data.Actor{ID: 1, Name: "Old", Aliases: pq.StringArray{}}, nil)
	repo.EXPECT().FindByNameOrAlias([]string{"Taken"}, uint(1)).Return([]data.Actor{{ID: 2, Name: "Taken"}}, nil)
	name := "Taken"
	if _, err := svc.Update(1, UpdateActorInput{Name: &name, Strict: true}); !apperrors.IsConflict(err) {
		t.Fatalf("want a conflict, got %v", err)
	}
}

func TestActorUpdateRenameToAliasDropsIt(t *testing.T) {
	svc, repo, _ := newTestActorService(t)
	repo.EXPECT().GetByID(uint(1)).Return(&data.Actor{ID: 1, Name: "Jane D", Aliases: pq.StringArray{"Jane Doe", "JD"}}, nil)
	repo.EXPECT().Update(gomock.Any()).Return(nil)
	// renamed: denormalized names updated for its scenes
	repo.EXPECT().GetActorSceneIDs(uint(1)).Return([]uint{}, nil)

	name := "Jane Doe"
	actor, err := svc.Update(1, UpdateActorInput{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if !sameStrings(actor.Aliases, []string{"JD"}) {
		t.Fatalf("aliases = %#v, want [JD]", actor.Aliases)
	}
}

func TestActorUpdateAliasesReindexes(t *testing.T) {
	svc, repo, sceneRepo := newTestActorService(t)
	idx := &recordingIndexer{}
	svc.SetIndexer(idx)
	repo.EXPECT().GetByID(uint(1)).Return(&data.Actor{ID: 1, Name: "Jane Doe", Aliases: pq.StringArray{}}, nil)
	repo.EXPECT().FindByNameOrAlias([]string{"JD"}, uint(1)).Return([]data.Actor{}, nil)
	repo.EXPECT().Update(gomock.Any()).Return(nil)
	repo.EXPECT().GetActorSceneIDs(uint(1)).Return([]uint{5}, nil)
	repo.EXPECT().GetSceneActors(uint(5)).Return([]data.Actor{{ID: 1, Name: "Jane Doe"}}, nil)
	sceneRepo.EXPECT().UpdateActors(uint(5), []string{"Jane Doe"}).Return(nil)
	sceneRepo.EXPECT().GetByID(uint(5)).Return(&data.Scene{ID: 5}, nil)

	aliases := []string{"JD"}
	if _, err := svc.Update(1, UpdateActorInput{Aliases: &aliases, Strict: true}); err != nil {
		t.Fatal(err)
	}
	if got := idx.calls(); len(got) != 1 || got[0] != 5 {
		t.Fatalf("re-indexed %v, want [5]", got)
	}
}

func TestActorMerge(t *testing.T) {
	svc, repo, sceneRepo := newTestActorService(t)
	idx := &bulkRecordingIndexer{}
	svc.SetIndexer(idx)

	target := &data.Actor{ID: 1, Name: "Jane Doe", Aliases: pq.StringArray{"JD"}}
	source := &data.Actor{ID: 2, Name: "Jane D.", Aliases: pq.StringArray{"jd", "Janie"}, ImageURL: "/actor-images/b.jpg"}
	repo.EXPECT().GetByID(uint(1)).Return(target, nil)
	repo.EXPECT().GetByID(uint(2)).Return(source, nil)
	repo.EXPECT().Merge(gomock.Any(), source).DoAndReturn(func(merged *data.Actor, _ *data.Actor) (*data.ActorMergeStats, []uint, error) {
		if !sameStrings(merged.Aliases, []string{"JD", "Jane D.", "Janie"}) {
			t.Fatalf("merged aliases = %#v", merged.Aliases)
		}
		if merged.ImageURL != "/actor-images/b.jpg" {
			t.Fatalf("the target has no photo: it takes the source's, got %q", merged.ImageURL)
		}
		return &data.ActorMergeStats{Scenes: 3, SharedScenes: 1, Ratings: 2, Likes: 1}, []uint{10, 11}, nil
	})
	sceneRepo.EXPECT().GetByIDs([]uint{10, 11}).Return([]data.Scene{{ID: 10}, {ID: 11}}, nil)

	res, err := svc.Merge(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scenes != 3 || res.Ratings != 2 || !res.ImageFromSource || !sameStrings(res.AliasesAdded, []string{"Jane D.", "Janie"}) {
		t.Fatalf("unexpected result %+v", res)
	}
	if len(idx.bulk) != 2 {
		t.Fatalf("re-indexed %v, want the target's scenes", idx.bulk)
	}
	if target.ImageURL != "" || len(target.Aliases) != 1 {
		t.Fatal("Merge must not modify the loaded target in place")
	}
}

func TestActorMergeKeepsTargetPhoto(t *testing.T) {
	target := &data.Actor{ID: 1, Name: "A", ImageURL: "/a.jpg"}
	source := &data.Actor{ID: 2, Name: "B", ImageURL: "/b.jpg"}
	merged, _, fromSource := mergedTarget(target, source)
	if merged.ImageURL != "/a.jpg" || fromSource {
		t.Fatalf("target photo must be kept, got %q (from source %v)", merged.ImageURL, fromSource)
	}
}

func TestActorMergeValidation(t *testing.T) {
	svc, repo, _ := newTestActorService(t)
	if _, err := svc.Merge(1, 1); !apperrors.IsValidation(err) {
		t.Fatalf("merging into itself: want a validation error, got %v", err)
	}
	repo.EXPECT().GetByID(uint(1)).Return(&data.Actor{ID: 1, Name: "A"}, nil)
	repo.EXPECT().GetByID(uint(2)).Return(nil, gorm.ErrRecordNotFound)
	if _, err := svc.Merge(1, 2); !apperrors.IsNotFound(err) {
		t.Fatalf("missing source: want not found, got %v", err)
	}
}

func TestActorMergePreview(t *testing.T) {
	svc, repo, _ := newTestActorService(t)
	repo.EXPECT().GetByID(uint(1)).Return(&data.Actor{ID: 1, Name: "A", ImageURL: "/a.jpg"}, nil)
	repo.EXPECT().GetByID(uint(2)).Return(&data.Actor{ID: 2, Name: "B"}, nil)
	repo.EXPECT().GetMergePreview(uint(1), uint(2)).Return(&data.ActorMergeStats{Scenes: 4}, nil)
	res, err := svc.MergePreview(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scenes != 4 || res.ImageFromSource || !sameStrings(res.AliasesAdded, []string{"B"}) {
		t.Fatalf("unexpected preview %+v", res)
	}
}
