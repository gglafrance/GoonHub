package core

import (
	"reflect"
	"testing"
	"time"

	"goonhub/internal/data"
	"goonhub/internal/infrastructure/meilisearch"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestSourceSiteFromURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"empty", "", ""},
		{"blank", "   ", ""},
		{"www", "https://www.lpsg.com/threads/some-thread.123/", "lpsg.com"},
		{"language subdomain", "https://fr.pornhub.com/view_video.php?viewkey=abc", "pornhub.com"},
		{"www variant", "https://www.pornhub.com/view_video.php?viewkey=abc", "pornhub.com"},
		{"mobile m", "https://m.pornhub.com/view_video.php?viewkey=abc", "pornhub.com"},
		{"mobile word", "https://mobile.example.com/v/1", "example.com"},
		{"language on xvideos", "https://fr.xvideos.com/video123/title", "xvideos.com"},
		{"no subdomain", "https://www.xvideos.com/video123/title", "xvideos.com"},
		{"bare domain", "https://xvideos.com/video123", "xvideos.com"},
		{"uppercase", "HTTPS://WWW.PornHub.COM/view", "pornhub.com"},
		{"port", "http://www.example.com:8080/v/1", "example.com"},
		{"trailing dot", "https://www.example.com./v/1", "example.com"},
		{"locale", "https://en-us.example.com/v/1", "example.com"},
		{"stacked variants", "https://de.m.pornhub.com/v", "pornhub.com"},
		{"www then language", "https://www.de.example.com/v", "example.com"},
		{"real subdomain kept", "https://player.vimeo.com/video/1", "player.vimeo.com"},
		{"three-letter label kept", "https://www.bbc.co.uk/news", "bbc.co.uk"},
		{"two labels kept", "https://ok.ru/video/1", "ok.ru"},
		{"variant over two labels", "https://m.ok.ru/video/1", "ok.ru"},
		{"never below two labels", "https://www.com/", "www.com"},
		{"ipv4", "http://192.168.1.10:9999/watch/1", "192.168.1.10"},
		{"ipv6", "http://[::1]:8080/x", "::1"},
		{"no host", "not a url", ""},
		{"path only", "/watch/1", ""},
		{"invalid", "http://%zz", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SourceSiteFromURL(tt.raw); got != tt.want {
				t.Fatalf("SourceSiteFromURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeSourceSite(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"", ""},
		{"  ", ""},
		{"pornhub.com", "pornhub.com"},
		{" www.PornHub.com ", "pornhub.com"},
		{"fr.xvideos.com", "xvideos.com"},
		{"https://fr.pornhub.com/view_video.php?viewkey=abc", "pornhub.com"},
		{"lpsg.com/threads/1", "lpsg.com"},
		{"example.com:8080", "example.com"},
	}
	for _, tt := range tests {
		if got := NormalizeSourceSite(tt.value); got != tt.want {
			t.Fatalf("NormalizeSourceSite(%q) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestParseSourceSites(t *testing.T) {
	tests := []struct {
		raw  string
		want []string
	}{
		{"", nil},
		{" , ", nil},
		{"pornhub.com", []string{"pornhub.com"}},
		{"www.PornHub.com,fr.xvideos.com,,pornhub.com", []string{"pornhub.com", "xvideos.com"}},
	}
	for _, tt := range tests {
		if got := ParseSourceSites(tt.raw); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("ParseSourceSites(%q) = %#v, want %#v", tt.raw, got, tt.want)
		}
	}
	if got := NormalizeSourceSites(nil); got != nil {
		t.Fatalf("NormalizeSourceSites(nil) = %#v, want nil", got)
	}
}

func TestSortSourceSiteCounts(t *testing.T) {
	got := sortSourceSiteCounts([]meilisearch.FacetCount{
		{Value: "xvideos.com", Count: 3},
		{Value: "lpsg.com", Count: 10},
		{Value: "anon.com", Count: 3},
	})
	want := []SourceSiteCount{
		{Site: "lpsg.com", Count: 10},
		{Site: "anon.com", Count: 3},
		{Site: "xvideos.com", Count: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortSourceSiteCounts = %#v, want %#v", got, want)
	}
	if got := sortSourceSiteCounts(nil); got == nil || len(got) != 0 {
		t.Fatalf("expected an empty, non-nil list, got %#v", got)
	}
}

func TestSearchService_GetSourceSites_RequiresMeilisearch(t *testing.T) {
	svc := NewSearchService(nil, nil, nil, nil, nil, nil, zap.NewNop())
	if _, err := svc.GetSourceSites(); err == nil {
		t.Fatal("expected an error without Meilisearch")
	}
}

func TestBuildSceneDocument_SourceSite(t *testing.T) {
	scene := &data.Scene{ID: 7, CreatedAt: time.Unix(100, 0), SourceURL: "https://fr.pornhub.com/view_video.php?viewkey=x"}
	if doc := buildSceneDocument(scene, nil, nil); doc.SourceSite != "pornhub.com" {
		t.Fatalf("source_site = %q, want pornhub.com", doc.SourceSite)
	}
	scene.SourceURL = ""
	if doc := buildSceneDocument(scene, nil, nil); doc.SourceSite != "" {
		t.Fatalf("source_site = %q, want empty", doc.SourceSite)
	}
}

func TestBuildMeiliParams_SourceFilters(t *testing.T) {
	svc := NewSearchService(nil, nil, nil, nil, nil, nil, zap.NewNop())
	no := false
	got := svc.buildMeiliParams(data.SceneSearchParams{
		Page:        1,
		Limit:       20,
		SourceSites: []string{"pornhub.com", "xvideos.com"},
		HasSource:   &no,
	}, nil)
	if !reflect.DeepEqual(got.SourceSites, []string{"pornhub.com", "xvideos.com"}) {
		t.Fatalf("SourceSites = %#v", got.SourceSites)
	}
	if got.HasSource == nil || *got.HasSource {
		t.Fatalf("HasSource = %v, want false", got.HasSource)
	}
}

func TestUpdateSceneSourceURL_Reindexes(t *testing.T) {
	ctrl := gomock.NewController(t)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	indexer := &recordingIndexer{}
	svc := &SceneService{Repo: sceneRepo, logger: zap.NewNop()}
	svc.SetIndexer(indexer)

	first := sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1}, nil)
	sceneRepo.EXPECT().UpdateSourceURL(uint(1), "https://www.xvideos.com/v/1").Return(nil)
	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1, SourceURL: "https://www.xvideos.com/v/1"}, nil).After(first)

	if _, err := svc.UpdateSceneSourceURL(1, "https://www.xvideos.com/v/1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := indexer.calls(); !reflect.DeepEqual(got, []uint{1}) {
		t.Fatalf("expected scene 1 to be reindexed once, got %v", got)
	}
}
