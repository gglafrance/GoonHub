package core

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/lib/pq"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type recordingIndexer struct {
	mu      sync.Mutex
	updated []uint
}

func (r *recordingIndexer) IndexScene(scene *data.Scene) error { return r.UpdateSceneIndex(scene) }
func (r *recordingIndexer) UpdateSceneIndex(scene *data.Scene) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updated = append(r.updated, scene.ID)
	return nil
}
func (r *recordingIndexer) BulkUpdateSceneIndex([]data.Scene) error { return nil }
func (r *recordingIndexer) DeleteSceneIndex(uint) error             { return nil }
func (r *recordingIndexer) BulkDeleteSceneIndex([]uint) error       { return nil }

func (r *recordingIndexer) calls() []uint {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]uint(nil), r.updated...)
}

type importTestDeps struct {
	importRepo      *mocks.MockImportRepository
	sceneRepo       *mocks.MockSceneRepository
	tagRepo         *mocks.MockTagRepository
	actorRepo       *mocks.MockActorRepository
	studioRepo      *mocks.MockStudioRepository
	storagePathRepo *mocks.MockStoragePathRepository
	userRepo        *mocks.MockUserRepository
	interactionRepo *mocks.MockInteractionRepository
	indexer         *recordingIndexer
}

func newTestImportService(t *testing.T) (*ImportService, *importTestDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := &importTestDeps{
		importRepo:      mocks.NewMockImportRepository(ctrl),
		sceneRepo:       mocks.NewMockSceneRepository(ctrl),
		tagRepo:         mocks.NewMockTagRepository(ctrl),
		actorRepo:       mocks.NewMockActorRepository(ctrl),
		studioRepo:      mocks.NewMockStudioRepository(ctrl),
		storagePathRepo: mocks.NewMockStoragePathRepository(ctrl),
		userRepo:        mocks.NewMockUserRepository(ctrl),
		interactionRepo: mocks.NewMockInteractionRepository(ctrl),
		indexer:         &recordingIndexer{},
	}
	svc := NewImportService(d.importRepo, d.sceneRepo, d.tagRepo, d.actorRepo, d.studioRepo,
		d.storagePathRepo, d.userRepo, d.interactionRepo, d.indexer, zap.NewNop())
	return svc, d
}

// expectTx makes RunInTransaction run its callback against the same mock, returning its error
// (which is what triggers a rollback in the real implementation).
func (d *importTestDeps) expectTx() {
	d.importRepo.EXPECT().RunInTransaction(gomock.Any()).DoAndReturn(func(fn func(data.ImportRepository) error) error {
		return fn(d.importRepo)
	})
}

func ptr[T any](v T) *T { return &v }

func baseSceneInput() ImportSceneInput {
	return ImportSceneInput{
		Title:         "Scene",
		StoredPath:    "/media/a.mp4",
		SkipFileCheck: true,
	}
}

func TestImportScene_Validation(t *testing.T) {
	long := strings.Repeat("x", 256)
	tests := []struct {
		name   string
		mutate func(*ImportSceneInput)
		want   string
	}{
		{"missing title", func(in *ImportSceneInput) { in.Title = "" }, "title is required"},
		{"title too long", func(in *ImportSceneInput) { in.Title = long }, "title must be at most"},
		{"missing stored_path", func(in *ImportSceneInput) { in.StoredPath = "" }, "stored_path is required"},
		{"bad release date", func(in *ImportSceneInput) { in.ReleaseDate = ptr("2024/01/02") }, "YYYY-MM-DD"},
		{"bad origin", func(in *ImportSceneInput) { in.Origin = "nope" }, "invalid origin"},
		{"bad type", func(in *ImportSceneInput) { in.Type = "nope" }, "invalid type"},
		{"negative view_count", func(in *ImportSceneInput) { in.ViewCount = ptr(int64(-1)) }, "view_count"},
		{"bad created_at", func(in *ImportSceneInput) { in.CreatedAt = ptr("2024-01-02") }, "RFC3339"},
		{"bad file_created_at", func(in *ImportSceneInput) { in.FileCreatedAt = ptr("yesterday") }, "RFC3339"},
		{"rating too low", func(in *ImportSceneInput) {
			in.Interactions = &ImportInteractionsInput{Rating: ptr(0.4)}
		}, "rating"},
		{"rating too high", func(in *ImportSceneInput) {
			in.Interactions = &ImportInteractionsInput{Rating: ptr(5.5)}
		}, "rating"},
		{"negative jizzed_count", func(in *ImportSceneInput) {
			in.Interactions = &ImportInteractionsInput{JizzedCount: ptr(-1)}
		}, "jizzed_count"},
		{"watch without watched_at", func(in *ImportSceneInput) {
			in.Interactions = &ImportInteractionsInput{Watches: []ImportWatchInput{{}}}
		}, "watched_at is required"},
		{"watch with bad watched_at", func(in *ImportSceneInput) {
			in.Interactions = &ImportInteractionsInput{Watches: []ImportWatchInput{{WatchedAt: "now"}}}
		}, "RFC3339"},
		{"missing file", func(in *ImportSceneInput) {
			in.SkipFileCheck = false
			in.StoredPath = "/definitely/not/here.mp4"
		}, "file not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No repository expectation: any write or lookup fails the test.
			svc, _ := newTestImportService(t)
			in := baseSceneInput()
			tt.mutate(&in)
			_, err := svc.ImportScene(1, in)
			if !apperrors.IsValidation(err) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %q", tt.want, err.Error())
			}
		})
	}
}

func TestImportScene_UnknownAssociationIDs(t *testing.T) {
	t.Run("tags", func(t *testing.T) {
		svc, d := newTestImportService(t)
		d.tagRepo.EXPECT().GetByIDs([]uint{1, 2, 3}).Return([]data.Tag{{ID: 1, Name: "a"}}, nil)
		in := baseSceneInput()
		in.TagIDs = []uint{1, 2, 2, 3}
		_, err := svc.ImportScene(1, in)
		if !apperrors.IsValidation(err) || !strings.Contains(err.Error(), "unknown tag_ids: [2 3]") {
			t.Fatalf("expected unknown tag_ids validation error, got %v", err)
		}
	})
	t.Run("actors", func(t *testing.T) {
		svc, d := newTestImportService(t)
		d.actorRepo.EXPECT().GetByIDs([]uint{7}).Return([]data.Actor{}, nil)
		in := baseSceneInput()
		in.ActorIDs = []uint{7}
		_, err := svc.ImportScene(1, in)
		if !apperrors.IsValidation(err) || !strings.Contains(err.Error(), "unknown actor_ids: [7]") {
			t.Fatalf("expected unknown actor_ids validation error, got %v", err)
		}
	})
	t.Run("studio", func(t *testing.T) {
		svc, d := newTestImportService(t)
		d.studioRepo.EXPECT().GetByID(uint(9)).Return(nil, gorm.ErrRecordNotFound)
		in := baseSceneInput()
		in.StudioID = ptr(uint(9))
		_, err := svc.ImportScene(1, in)
		if !apperrors.IsValidation(err) || !strings.Contains(err.Error(), "unknown studio_id: 9") {
			t.Fatalf("expected unknown studio_id validation error, got %v", err)
		}
	})
}

func TestImportScene_CreateWithAssociationsAndInteractions(t *testing.T) {
	svc, d := newTestImportService(t)

	d.tagRepo.EXPECT().GetByIDs([]uint{2, 1}).Return([]data.Tag{{ID: 1, Name: "b-tag"}, {ID: 2, Name: "a-tag"}}, nil)
	d.actorRepo.EXPECT().GetByIDs([]uint{5}).Return([]data.Actor{{ID: 5, Name: "Actor"}}, nil)
	d.studioRepo.EXPECT().GetByID(uint(3)).Return(&data.Studio{ID: 3, Name: "Studio"}, nil)
	d.sceneRepo.EXPECT().GetByStoredPath("/media/a.mp4").Return(nil, gorm.ErrRecordNotFound)
	d.expectTx()

	createdAt := "2020-05-06T07:08:09Z"
	var createdScene *data.Scene
	gomock.InOrder(
		d.importRepo.EXPECT().CreateScene(gomock.Any()).DoAndReturn(func(s *data.Scene) error {
			s.ID = 42
			createdScene = s
			return nil
		}),
		d.importRepo.EXPECT().SetSceneTags(uint(42), []uint{2, 1}).Return(nil),
		d.importRepo.EXPECT().SetSceneActors(uint(42), []uint{5}).Return(nil),
		d.importRepo.EXPECT().UpsertRating(uint(1), uint(42), 4.5).Return(nil),
		d.importRepo.EXPECT().SetLiked(uint(1), uint(42), true).Return(nil),
		d.importRepo.EXPECT().SetJizzedCount(uint(1), uint(42), 3).Return(nil),
		d.importRepo.EXPECT().InsertWatchIfAbsent(gomock.Any()).DoAndReturn(func(w *data.UserSceneWatch) (bool, error) {
			if w.UserID != 1 || w.SceneID != 42 || w.WatchDuration != 60 || w.LastPosition != 30 || !w.Completed {
				t.Fatalf("unexpected watch: %+v", w)
			}
			return true, nil
		}),
	)
	d.sceneRepo.EXPECT().GetByID(uint(42)).Return(&data.Scene{ID: 42}, nil)

	in := baseSceneInput()
	in.TagIDs = []uint{2, 1}
	in.ActorIDs = []uint{5}
	in.StudioID = ptr(uint(3))
	in.ViewCount = ptr(int64(7))
	in.CreatedAt = &createdAt
	in.ReleaseDate = ptr("2019-01-02")
	in.Origin = data.SceneOriginStash
	in.Interactions = &ImportInteractionsInput{
		Rating:      ptr(4.4), // rounded to 4.5
		Liked:       ptr(true),
		JizzedCount: ptr(3),
		Watches:     []ImportWatchInput{{WatchedAt: "2021-01-01T10:00:00Z", WatchDuration: 60, LastPosition: 30, Completed: true}},
	}

	res, err := svc.ImportScene(1, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != 42 || !res.Created || res.Title != "Scene" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if createdScene.ProcessingStatus != "completed" || createdScene.OriginalFilename != "a.mp4" {
		t.Fatalf("unexpected scene defaults: %+v", createdScene)
	}
	if createdScene.ViewCount != 7 || createdScene.StudioID == nil || *createdScene.StudioID != 3 || createdScene.Studio != "Studio" {
		t.Fatalf("unexpected scene fields: %+v", createdScene)
	}
	if want, _ := time.Parse(time.RFC3339, createdAt); !createdScene.CreatedAt.Equal(want) {
		t.Fatalf("expected created_at %v, got %v", want, createdScene.CreatedAt)
	}
	if strings.Join(createdScene.Tags, ",") != "a-tag,b-tag" || strings.Join(createdScene.Actors, ",") != "Actor" {
		t.Fatalf("denormalized fields not synced: tags=%v actors=%v", createdScene.Tags, createdScene.Actors)
	}
	if got := d.indexer.calls(); len(got) != 1 || got[0] != 42 {
		t.Fatalf("expected scene 42 to be indexed once, got %v", got)
	}
}

func TestImportScene_SkipIndex(t *testing.T) {
	svc, d := newTestImportService(t)
	d.sceneRepo.EXPECT().GetByStoredPath("/media/a.mp4").Return(nil, gorm.ErrRecordNotFound)
	d.expectTx()
	d.importRepo.EXPECT().CreateScene(gomock.Any()).DoAndReturn(func(s *data.Scene) error { s.ID = 1; return nil })

	in := baseSceneInput()
	in.SkipIndex = true
	in.TagIDs = []uint{} // explicit empty on create: nothing to write
	if _, err := svc.ImportScene(1, in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := d.indexer.calls(); len(got) != 0 {
		t.Fatalf("expected no indexing, got %v", got)
	}
}

func TestImportScene_ExistingConflict(t *testing.T) {
	svc, d := newTestImportService(t)
	d.sceneRepo.EXPECT().GetByStoredPath("/media/a.mp4").Return(&data.Scene{ID: 10, Title: "Old"}, nil)

	_, err := svc.ImportScene(1, baseSceneInput())
	var conflict *ImportConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ImportConflictError, got %v", err)
	}
	if conflict.ID != 10 || conflict.Title != "Old" {
		t.Fatalf("unexpected conflict payload: %+v", conflict)
	}
	if !apperrors.IsConflict(err) {
		t.Fatal("expected error to unwrap to an apperrors ConflictError")
	}
}

func TestImportScene_UpdateExisting(t *testing.T) {
	svc, d := newTestImportService(t)
	d.tagRepo.EXPECT().GetByIDs([]uint{4}).Return([]data.Tag{{ID: 4, Name: "t"}}, nil)
	d.sceneRepo.EXPECT().GetByStoredPath("/media/a.mp4").Return(&data.Scene{ID: 10, Title: "Old"}, nil)
	d.expectTx()

	var fields map[string]any
	d.importRepo.EXPECT().UpdateSceneFields(uint(10), gomock.Any()).DoAndReturn(func(_ uint, f map[string]any) error {
		fields = f
		return nil
	})
	d.importRepo.EXPECT().SetSceneTags(uint(10), []uint{4}).Return(nil)
	d.importRepo.EXPECT().SetLiked(uint(1), uint(10), false).Return(nil)
	d.sceneRepo.EXPECT().GetByID(uint(10)).Return(&data.Scene{ID: 10}, nil)

	in := baseSceneInput()
	in.Title = "New"
	in.UpdateExisting = true
	in.TagIDs = []uint{4}
	in.Duration = 120
	in.ViewCount = ptr(int64(2))
	in.CreatedAt = ptr("2020-01-01T00:00:00Z") // ignored on update
	in.ReleaseDate = ptr("")                   // explicit clear
	in.Interactions = &ImportInteractionsInput{Liked: ptr(false), Rating: ptr(0.0)}

	res, err := svc.ImportScene(1, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != 10 || res.Created || res.Title != "New" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if fields["title"] != "New" || fields["duration"] != 120 || fields["view_count"] != int64(2) {
		t.Fatalf("unexpected update fields: %v", fields)
	}
	if v, ok := fields["release_date"]; !ok || v != nil {
		t.Fatalf("expected release_date to be cleared, got %v (present=%v)", v, ok)
	}
	if tags, ok := fields["tags"].(pq.StringArray); !ok || len(tags) != 1 || tags[0] != "t" {
		t.Fatalf("expected denormalized tags to be synced, got %v", fields["tags"])
	}
	for _, k := range []string{"created_at", "width", "size", "description", "studio_id", "actors", "origin"} {
		if _, ok := fields[k]; ok {
			t.Fatalf("field %q should not be updated, got %v", k, fields)
		}
	}
}

func TestImportScene_RollbackOnAssociationFailure(t *testing.T) {
	svc, d := newTestImportService(t)
	d.tagRepo.EXPECT().GetByIDs([]uint{1}).Return([]data.Tag{{ID: 1, Name: "a"}}, nil)
	d.actorRepo.EXPECT().GetByIDs([]uint{2}).Return([]data.Actor{{ID: 2, Name: "b"}}, nil)
	d.sceneRepo.EXPECT().GetByStoredPath("/media/a.mp4").Return(nil, gorm.ErrRecordNotFound)

	var txErr error
	d.importRepo.EXPECT().RunInTransaction(gomock.Any()).DoAndReturn(func(fn func(data.ImportRepository) error) error {
		txErr = fn(d.importRepo)
		return txErr
	})
	d.importRepo.EXPECT().CreateScene(gomock.Any()).DoAndReturn(func(s *data.Scene) error { s.ID = 5; return nil })
	d.importRepo.EXPECT().SetSceneTags(uint(5), []uint{1}).Return(errors.New("fk violation"))
	// No SetSceneActors / interactions / index calls are expected after the failure.

	in := baseSceneInput()
	in.TagIDs = []uint{1}
	in.ActorIDs = []uint{2}
	in.Interactions = &ImportInteractionsInput{Liked: ptr(true)}

	_, err := svc.ImportScene(1, in)
	if !apperrors.IsInternal(err) {
		t.Fatalf("expected internal error, got %v", err)
	}
	if txErr == nil {
		t.Fatal("expected the transaction callback to return an error so the tx is rolled back")
	}
	if got := d.indexer.calls(); len(got) != 0 {
		t.Fatalf("expected no indexing after rollback, got %v", got)
	}
}

func TestImportMarker_Validation(t *testing.T) {
	tests := []struct {
		name string
		in   ImportMarkerInput
		want string
	}{
		{"missing scene", ImportMarkerInput{}, "scene_id is required"},
		{"negative timestamp", ImportMarkerInput{SceneID: 1, Timestamp: -1}, "timestamp"},
		{"label too long", ImportMarkerInput{SceneID: 1, Label: strings.Repeat("é", 101)}, "label must be at most 100"},
		{"bad color", ImportMarkerInput{SceneID: 1, Color: "red"}, "color"},
		{"end equal to start", ImportMarkerInput{SceneID: 1, Timestamp: 10, EndTimestamp: ptr(10)}, "end_timestamp"},
		{"end before start", ImportMarkerInput{SceneID: 1, Timestamp: 10, EndTimestamp: ptr(5)}, "end_timestamp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestImportService(t)
			_, err := svc.ImportMarker(1, tt.in)
			if !apperrors.IsValidation(err) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected validation error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestImportMarker_LabelOf100RunesAccepted(t *testing.T) {
	svc, d := newTestImportService(t)
	label := strings.Repeat("é", 100)
	d.sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1}, nil)
	d.expectTx()
	d.importRepo.EXPECT().FindMarker(uint(1), uint(1), 0, label).Return(nil, nil)
	d.importRepo.EXPECT().CreateMarker(gomock.Any()).DoAndReturn(func(m *data.UserSceneMarker) error { m.ID = 3; return nil })
	d.importRepo.EXPECT().ApplyLabelTagsToMarker(uint(1), uint(3), label).Return(nil)

	if _, err := svc.ImportMarker(1, ImportMarkerInput{SceneID: 1, Label: label}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportMarker_CreateDefaultsToCurrentUser(t *testing.T) {
	svc, d := newTestImportService(t)
	d.tagRepo.EXPECT().GetByIDs([]uint{8}).Return([]data.Tag{{ID: 8, Name: "x"}}, nil)
	d.sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2}, nil)
	d.expectTx()
	var created *data.UserSceneMarker
	gomock.InOrder(
		d.importRepo.EXPECT().FindMarker(uint(7), uint(2), 30, "Intro").Return(nil, nil),
		d.importRepo.EXPECT().CreateMarker(gomock.Any()).DoAndReturn(func(m *data.UserSceneMarker) error {
			m.ID = 11
			created = m
			return nil
		}),
		d.importRepo.EXPECT().ApplyLabelTagsToMarker(uint(7), uint(11), "Intro").Return(nil),
		d.importRepo.EXPECT().AddMarkerTags(uint(11), []uint{8}).Return(nil),
	)

	res, err := svc.ImportMarker(7, ImportMarkerInput{SceneID: 2, Timestamp: 30, Label: "Intro", TagIDs: []uint{8}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != 11 || res.SceneID != 2 || !res.Created {
		t.Fatalf("unexpected result: %+v", res)
	}
	if created.UserID != 7 || created.Color != "#FFFFFF" {
		t.Fatalf("unexpected marker: %+v", created)
	}
}

func TestImportMarker_ExplicitUser(t *testing.T) {
	t.Run("unknown user", func(t *testing.T) {
		svc, d := newTestImportService(t)
		d.userRepo.EXPECT().GetByID(uint(99)).Return(nil, gorm.ErrRecordNotFound)
		_, err := svc.ImportMarker(1, ImportMarkerInput{SceneID: 2, UserID: ptr(uint(99))})
		if !apperrors.IsValidation(err) || !strings.Contains(err.Error(), "unknown user_id") {
			t.Fatalf("expected unknown user_id validation error, got %v", err)
		}
	})
	t.Run("scene not found", func(t *testing.T) {
		svc, d := newTestImportService(t)
		d.userRepo.EXPECT().GetByID(uint(5)).Return(&data.User{ID: 5}, nil)
		d.sceneRepo.EXPECT().GetByID(uint(2)).Return(nil, gorm.ErrRecordNotFound)
		_, err := svc.ImportMarker(1, ImportMarkerInput{SceneID: 2, UserID: ptr(uint(5))})
		if !apperrors.IsNotFound(err) {
			t.Fatalf("expected not found error, got %v", err)
		}
	})
}

func TestImportMarker_DedupConflict(t *testing.T) {
	svc, d := newTestImportService(t)
	d.sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2}, nil)
	d.expectTx()
	d.importRepo.EXPECT().FindMarker(uint(1), uint(2), 30, "Intro").
		Return(&data.UserSceneMarker{ID: 11, SceneID: 2, Color: "#FFFFFF"}, nil)

	_, err := svc.ImportMarker(1, ImportMarkerInput{SceneID: 2, Timestamp: 30, Label: "Intro", Color: "#FF0000"})
	var conflict *ImportConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ImportConflictError, got %v", err)
	}
	if conflict.ID != 11 || conflict.SceneID != 2 {
		t.Fatalf("unexpected conflict payload: %+v", conflict)
	}
}

func TestImportMarker_DedupUpdateExisting(t *testing.T) {
	svc, d := newTestImportService(t)
	d.tagRepo.EXPECT().GetByIDs([]uint{8}).Return([]data.Tag{{ID: 8, Name: "x"}}, nil)
	d.sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2}, nil)
	d.expectTx()
	gomock.InOrder(
		d.importRepo.EXPECT().FindMarker(uint(1), uint(2), 30, "Intro").
			Return(&data.UserSceneMarker{ID: 11, SceneID: 2, Color: "#FFFFFF"}, nil),
		d.importRepo.EXPECT().UpdateMarkerColor(uint(11), "#FF0000").Return(nil),
		d.importRepo.EXPECT().ApplyLabelTagsToMarker(uint(1), uint(11), "Intro").Return(nil),
		d.importRepo.EXPECT().SetMarkerTags(uint(11), []uint{8}).Return(nil),
	)

	res, err := svc.ImportMarker(1, ImportMarkerInput{
		SceneID: 2, Timestamp: 30, Label: "Intro", Color: "#FF0000", TagIDs: []uint{8}, UpdateExisting: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != 11 || res.Created {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestImportMarker_CreateWithEnd(t *testing.T) {
	svc, d := newTestImportService(t)
	d.sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2}, nil)
	d.expectTx()
	var created *data.UserSceneMarker
	d.importRepo.EXPECT().FindMarker(uint(1), uint(2), 30, "Intro").Return(nil, nil)
	d.importRepo.EXPECT().CreateMarker(gomock.Any()).DoAndReturn(func(m *data.UserSceneMarker) error {
		m.ID = 12
		created = m
		return nil
	})
	d.importRepo.EXPECT().ApplyLabelTagsToMarker(uint(1), uint(12), "Intro").Return(nil)

	if _, err := svc.ImportMarker(1, ImportMarkerInput{SceneID: 2, Timestamp: 30, EndTimestamp: ptr(45), Label: "Intro"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.EndTimestamp == nil || *created.EndTimestamp != 45 {
		t.Fatalf("expected end_timestamp 45, got %v", created.EndTimestamp)
	}
}

func TestImportMarker_UpdateExistingEnd(t *testing.T) {
	tests := []struct {
		name       string
		existing   *int
		in         *int
		wantUpdate bool
	}{
		{"set missing end", nil, ptr(45), true},
		{"change end", ptr(40), ptr(45), true},
		{"same end", ptr(45), ptr(45), false},
		{"absent end keeps existing", ptr(45), nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, d := newTestImportService(t)
			d.sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2}, nil)
			d.expectTx()
			d.importRepo.EXPECT().FindMarker(uint(1), uint(2), 30, "Intro").
				Return(&data.UserSceneMarker{ID: 11, SceneID: 2, Color: "#FFFFFF", EndTimestamp: tt.existing}, nil)
			if tt.wantUpdate {
				d.importRepo.EXPECT().UpdateMarkerEndTimestamp(uint(11), tt.in).Return(nil)
			}
			d.importRepo.EXPECT().ApplyLabelTagsToMarker(uint(1), uint(11), "Intro").Return(nil)

			res, err := svc.ImportMarker(1, ImportMarkerInput{
				SceneID: 2, Timestamp: 30, EndTimestamp: tt.in, Label: "Intro", UpdateExisting: true,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.ID != 11 || res.Created {
				t.Fatalf("unexpected result: %+v", res)
			}
		})
	}
}

func TestImportSummary_TooManyIDs(t *testing.T) {
	svc, _ := newTestImportService(t)
	ids := make([]uint, MaxImportSummaryIDs+1)
	_, err := svc.Summary(1, ids)
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestImportSummary_MissingAndData(t *testing.T) {
	svc, d := newTestImportService(t)
	trashedAt := time.Now()
	studioID := uint(3)
	d.importRepo.EXPECT().GetScenesForSummary([]uint{1, 2, 3}).Return([]data.Scene{
		{ID: 2, Title: "two", StoredPath: "/b", TrashedAt: &trashedAt},
		{ID: 1, Title: "one", StoredPath: "/a", StudioID: &studioID, ViewCount: 4},
	}, nil)
	found := gomock.InAnyOrder([]uint{2, 1})
	d.tagRepo.EXPECT().GetSceneTagsMultiple(found).Return(map[uint][]data.Tag{1: {{ID: 1}, {ID: 2}}}, nil)
	d.actorRepo.EXPECT().GetSceneActorsMultiple(found).Return(map[uint][]data.Actor{1: {{ID: 5}}}, nil)
	d.interactionRepo.EXPECT().GetRatingsBySceneIDs(uint(9), found).Return(map[uint]float64{1: 4.5}, nil)
	d.interactionRepo.EXPECT().GetLikesBySceneIDs(uint(9), found).Return(map[uint]bool{2: true}, nil)
	d.interactionRepo.EXPECT().GetJizzCountsBySceneIDs(uint(9), found).Return(map[uint]int{1: 1}, nil)
	d.importRepo.EXPECT().CountMarkersByScenes(uint(9), found).Return(map[uint]int64{1: 2}, nil)
	d.importRepo.EXPECT().CountWatchesByScenes(uint(9), found).Return(map[uint]int64{1: 3}, nil)

	res, err := svc.Summary(9, []uint{1, 2, 3, 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Missing) != 1 || res.Missing[0] != 3 {
		t.Fatalf("expected missing [3], got %v", res.Missing)
	}
	if len(res.Scenes) != 2 || res.Scenes[0].ID != 1 || res.Scenes[1].ID != 2 {
		t.Fatalf("expected scenes in request order [1 2], got %+v", res.Scenes)
	}
	one, two := res.Scenes[0], res.Scenes[1]
	if one.StudioID == nil || *one.StudioID != 3 || len(one.TagIDs) != 2 || len(one.ActorIDs) != 1 ||
		one.ViewCount != 4 || one.MarkerCount != 2 || one.Rating != 4.5 || one.Liked || one.JizzedCount != 1 ||
		one.WatchCount != 3 || one.Trashed {
		t.Fatalf("unexpected summary for scene 1: %+v", one)
	}
	if !two.Trashed || !two.Liked || two.TagIDs == nil || len(two.TagIDs) != 0 || two.ActorIDs == nil {
		t.Fatalf("unexpected summary for scene 2: %+v", two)
	}
}

func TestImportSummary_Empty(t *testing.T) {
	svc, _ := newTestImportService(t)
	res, err := svc.Summary(1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Scenes == nil || res.Missing == nil || len(res.Scenes) != 0 || len(res.Missing) != 0 {
		t.Fatalf("expected empty non-nil slices, got %+v", res)
	}
}
