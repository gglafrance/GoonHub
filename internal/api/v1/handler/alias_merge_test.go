package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newAliasTestRouter(t *testing.T) (*gin.Engine, *mocks.MockActorRepository, *mocks.MockStudioRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	actorRepo := mocks.NewMockActorRepository(ctrl)
	studioRepo := mocks.NewMockStudioRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	ah := NewActorHandler(core.NewActorService(actorRepo, sceneRepo, zap.NewNop()), t.TempDir(), 100)
	sh := NewStudioHandler(core.NewStudioService(studioRepo, sceneRepo, zap.NewNop()), t.TempDir(), 100)

	r := gin.New()
	// same shape as routes.go: the static /resolve next to /:uuid
	r.GET("/actors", ah.ListActors)
	r.GET("/actors/resolve", ah.ResolveActor)
	r.GET("/actors/:uuid", ah.GetActorByUUID)
	r.GET("/actors/:uuid/merge-preview", ah.MergeActorPreview)
	r.POST("/actors/:uuid/merge", ah.MergeActor)
	r.POST("/admin/actors", ah.CreateActor)
	r.GET("/studios", sh.ListStudios)
	r.GET("/studios/resolve", sh.ResolveStudio)
	r.GET("/studios/:uuid", sh.GetStudioByUUID)
	r.POST("/admin/studios", sh.CreateStudio)
	r.POST("/studios/:uuid/merge", sh.MergeStudio)
	return r, actorRepo, studioRepo
}

func doAliasRequest(t *testing.T, r *gin.Engine, method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("invalid JSON %q: %v", w.Body.String(), err)
		}
	}
	return w, out
}

func TestResolveActorEndpoint(t *testing.T) {
	r, actorRepo, _ := newAliasTestRouter(t)
	actorRepo.EXPECT().FindByNameOrAlias([]string{"JD"}, uint(0)).Return([]data.Actor{
		{ID: 3, UUID: uuid.New(), Name: "Jane Doe", Aliases: pq.StringArray{"jd"}},
	}, nil)
	w, body := doAliasRequest(t, r, http.MethodGet, "/actors/resolve?name=JD", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	entity := body["data"].(map[string]any)
	if entity["name"] != "Jane Doe" || body["matched"] != "alias" || body["matched_alias"] != "jd" || body["candidates"] != float64(1) {
		t.Fatalf("unexpected body %v", body)
	}

	actorRepo.EXPECT().FindByNameOrAlias([]string{"Nobody"}, uint(0)).Return(nil, nil)
	if w, _ := doAliasRequest(t, r, http.MethodGet, "/actors/resolve?name=Nobody", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown name: status %d, want 404", w.Code)
	}
	if w, _ := doAliasRequest(t, r, http.MethodGet, "/actors/resolve", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("no name: status %d, want 400", w.Code)
	}
}

func TestResolveStudioEndpoint(t *testing.T) {
	r, _, studioRepo := newAliasTestRouter(t)
	studioRepo.EXPECT().FindByNameOrAlias([]string{"brazzers"}, uint(0)).Return([]data.Studio{{ID: 4, Name: "Brazzers"}}, nil)
	w, body := doAliasRequest(t, r, http.MethodGet, "/studios/resolve?name=brazzers", "")
	if w.Code != http.StatusOK || body["matched"] != "name" || body["data"].(map[string]any)["name"] != "Brazzers" {
		t.Fatalf("status %d body %v", w.Code, body)
	}
}

func TestCreateActorStrictConflict(t *testing.T) {
	r, actorRepo, _ := newAliasTestRouter(t)
	existingUUID := uuid.New()
	actorRepo.EXPECT().FindByNameOrAlias([]string{"Jane Doe"}, uint(0)).Return([]data.Actor{
		{ID: 3, UUID: existingUUID, Name: "jane doe"},
	}, nil)
	w, body := doAliasRequest(t, r, http.MethodPost, "/admin/actors?strict=true", `{"name":"Jane Doe"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", w.Code, w.Body.String())
	}
	existing := body["existing"].(map[string]any)
	if body["field"] != "name" || body["code"] != "CONFLICT" || existing["uuid"] != existingUUID.String() {
		t.Fatalf("unexpected body %v", body)
	}
}

func TestCreateActorWithoutStrictAllowsDuplicates(t *testing.T) {
	r, actorRepo, _ := newAliasTestRouter(t)
	// no duplicate check: the import contract creates same-name actors
	actorRepo.EXPECT().Create(gomock.Any()).Return(nil)
	w, body := doAliasRequest(t, r, http.MethodPost, "/admin/actors", `{"name":"Jane Doe"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if aliases, ok := body["aliases"].([]any); !ok || len(aliases) != 0 {
		t.Fatalf("aliases must be an empty array, got %v", body["aliases"])
	}
}

func TestCreateStudioStrictAliasConflict(t *testing.T) {
	r, _, studioRepo := newAliasTestRouter(t)
	studioRepo.EXPECT().FindByNameOrAlias([]string{"New"}, uint(0)).Return(nil, nil)
	studioRepo.EXPECT().FindByNameOrAlias([]string{"BZ"}, uint(0)).Return([]data.Studio{{ID: 4, Name: "Brazzers", Aliases: pq.StringArray{"BZ"}}}, nil)
	w, body := doAliasRequest(t, r, http.MethodPost, "/admin/studios?strict=1", `{"name":"New","aliases":["BZ"]}`)
	if w.Code != http.StatusConflict || body["field"] != "aliases" || body["value"] != "BZ" {
		t.Fatalf("status %d body %v", w.Code, body)
	}
}

func TestListActorsShowsMatchedAlias(t *testing.T) {
	r, actorRepo, _ := newAliasTestRouter(t)
	actorRepo.EXPECT().Search("janie", 1, 20, "", gomock.Nil()).Return([]data.ActorWithCount{
		{Actor: data.Actor{ID: 1, Name: "Jane Doe", Aliases: pq.StringArray{"JD", "Janie D"}}},
		{Actor: data.Actor{ID: 2, Name: "Janie Smith"}},
	}, int64(2), nil)
	w, body := doAliasRequest(t, r, http.MethodGet, "/actors?q=janie", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	items := body["data"].([]any)
	if got := items[0].(map[string]any)["matched_alias"]; got != "Janie D" {
		t.Fatalf("first result matched_alias = %v", got)
	}
	if _, present := items[1].(map[string]any)["matched_alias"]; present {
		t.Fatal("a name match has no matched_alias")
	}
}

func TestListStudiosIncludesAliases(t *testing.T) {
	r, _, studioRepo := newAliasTestRouter(t)
	studioRepo.EXPECT().Search("bz", 1, 20, "").Return([]data.StudioWithCount{
		{Studio: data.Studio{ID: 4, Name: "Brazzers", Aliases: pq.StringArray{"BZ"}}},
	}, int64(1), nil)
	_, body := doAliasRequest(t, r, http.MethodGet, "/studios?q=bz", "")
	item := body["data"].([]any)[0].(map[string]any)
	if item["matched_alias"] != "BZ" || len(item["aliases"].([]any)) != 1 {
		t.Fatalf("unexpected item %v", item)
	}
}

func TestMergeActorEndpoint(t *testing.T) {
	r, actorRepo, _ := newAliasTestRouter(t)
	targetUUID, sourceUUID := uuid.New(), uuid.New()
	target := &data.Actor{ID: 1, UUID: targetUUID, Name: "Jane Doe"}
	source := &data.Actor{ID: 2, UUID: sourceUUID, Name: "Jane D."}
	actorRepo.EXPECT().GetByUUID(targetUUID.String()).Return(target, nil)
	actorRepo.EXPECT().GetByUUID(sourceUUID.String()).Return(source, nil)
	actorRepo.EXPECT().GetSceneCount(gomock.Any()).Return(int64(0), nil).Times(2)
	actorRepo.EXPECT().GetByID(uint(1)).Return(target, nil)
	actorRepo.EXPECT().GetByID(uint(2)).Return(source, nil)
	actorRepo.EXPECT().Merge(gomock.Any(), gomock.Any()).Return(&data.ActorMergeStats{Scenes: 5}, nil, nil)

	w, body := doAliasRequest(t, r, http.MethodPost, "/actors/"+targetUUID.String()+"/merge", `{"source_uuid":"`+sourceUUID.String()+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if body["scenes"] != float64(5) || body["target"].(map[string]any)["name"] != "Jane Doe" {
		t.Fatalf("unexpected body %v", body)
	}
	added := body["aliases_added"].([]any)
	if len(added) != 1 || added[0] != "Jane D." {
		t.Fatalf("aliases_added = %v", added)
	}

	if w, _ := doAliasRequest(t, r, http.MethodPost, "/actors/"+targetUUID.String()+"/merge", `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("missing source: status %d, want 400", w.Code)
	}
}

func TestMergeStudioEndpointSelf(t *testing.T) {
	r, _, studioRepo := newAliasTestRouter(t)
	id := uuid.New()
	studio := &data.Studio{ID: 1, UUID: id, Name: "S"}
	studioRepo.EXPECT().GetByUUID(id.String()).Return(studio, nil).Times(2)
	studioRepo.EXPECT().GetSceneCount(uint(1)).Return(int64(0), nil).Times(2)
	w, _ := doAliasRequest(t, r, http.MethodPost, "/studios/"+id.String()+"/merge", `{"source_uuid":"`+id.String()+`"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("merging a studio into itself: status %d, want 400", w.Code)
	}

	other := uuid.New()
	studioRepo.EXPECT().GetByUUID(id.String()).Return(studio, nil)
	studioRepo.EXPECT().GetSceneCount(uint(1)).Return(int64(0), nil)
	studioRepo.EXPECT().GetByUUID(other.String()).Return(nil, gorm.ErrRecordNotFound)
	if w, _ := doAliasRequest(t, r, http.MethodPost, "/studios/"+id.String()+"/merge", `{"source_uuid":"`+other.String()+`"}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown source: status %d, want 404", w.Code)
	}
}
