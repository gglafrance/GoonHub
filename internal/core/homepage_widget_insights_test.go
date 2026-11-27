package core

import (
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestInsightsWidgetsAreRegistered(t *testing.T) {
	for _, widgetType := range []string{"trends", "performer_spotlight", "curation_todo", "processing_status", "saved_search_tiles"} {
		if !IsHomepageWidgetType(widgetType) {
			t.Fatalf("widget %q is not registered", widgetType)
		}
	}
}

func TestInsightsWidgetValidation(t *testing.T) {
	tests := []struct {
		name    string
		widget  string
		cfg     map[string]interface{}
		wantErr bool
	}{
		{"trends defaults", "trends", map[string]interface{}{}, false},
		{"trends valid", "trends", map[string]interface{}{"kind": "tags", "recent_days": float64(14), "baseline_days": float64(90), "min_views": float64(3)}, false},
		{"trends bad kind", "trends", map[string]interface{}{"kind": "studios"}, true},
		{"trends recent too long", "trends", map[string]interface{}{"recent_days": float64(400)}, true},
		{"trends baseline zero", "trends", map[string]interface{}{"baseline_days": float64(0)}, true},
		{"trends kind not a string", "trends", map[string]interface{}{"kind": float64(1)}, true},
		{"performers default", "performer_spotlight", map[string]interface{}{}, false},
		{"performers jizz", "performer_spotlight", map[string]interface{}{"order": "jizz"}, false},
		{"performers bad order", "performer_spotlight", map[string]interface{}{"order": "rating"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := data.HomepageSection{Type: tt.widget, Config: tt.cfg}
			err := validateHomepageWidgetConfig(&section)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInsightsRankTrends(t *testing.T) {
	totals := data.InsightsViewTotals{Recent: 100, Base: 1000}
	rows := []data.InsightsTrendRow{
		{ID: 1, Name: "rising", RecentViews: 30, BaseViews: 100}, // 30% vs 10%
		{ID: 2, Name: "falling", RecentViews: 5, BaseViews: 300}, // 5% vs 30%
		{ID: 3, Name: "new", RecentViews: 10, BaseViews: 0},      // 10% vs 0%
		{ID: 4, Name: "too few", RecentViews: 1, BaseViews: 0},   // below min views
		{ID: 5, Name: "slight", RecentViews: 12, BaseViews: 110}, // 12% vs 11%
	}
	items := insightsTrendItems("tag", rows, totals)
	ranked := insightsRankTrends(items, 2, 10)

	names := make([]string, len(ranked))
	for i, it := range ranked {
		names[i] = it.Name
	}
	want := []string{"rising", "new", "slight"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v, want %v", names, want)
		}
	}
	if ranked[0].ChangePct == nil || *ranked[0].ChangePct < 199 || *ranked[0].ChangePct > 201 {
		t.Fatalf("expected +200%% for 'rising', got %v", ranked[0].ChangePct)
	}
	if !ranked[1].New || ranked[1].ChangePct != nil {
		t.Fatalf("'new' must be flagged new without change %%: %+v", ranked[1])
	}
	if got := insightsRankTrends(items, 2, 1); len(got) != 1 {
		t.Fatalf("limit not applied: %d items", len(got))
	}
}

func TestInsightsRankTrends_NoViews(t *testing.T) {
	items := insightsTrendItems("actor", []data.InsightsTrendRow{{ID: 1, Name: "x"}}, data.InsightsViewTotals{})
	if got := insightsRankTrends(items, 1, 5); len(got) != 0 {
		t.Fatalf("expected no trend without views, got %+v", got)
	}
}

func TestInsightsGroupPhases(t *testing.T) {
	phases := insightsGroupPhases([]data.InsightsJobCount{
		{Phase: "sprites", Status: "pending", Count: 5},
		{Phase: "fingerprint", Status: "running", Count: 2},
		{Phase: "sprites", Status: "running", Count: 1},
		{Phase: "thumbnail", Status: "failed", Count: 3},
		{Phase: "custom", Status: "pending", Count: 1},
	})
	order := []string{"thumbnail", "sprites", "fingerprint", "custom"}
	if len(phases) != len(order) {
		t.Fatalf("unexpected phases: %+v", phases)
	}
	for i, p := range order {
		if phases[i].Phase != p {
			t.Fatalf("phase %d = %s, want %s (%+v)", i, phases[i].Phase, p, phases)
		}
	}
	if phases[1].Pending != 5 || phases[1].Running != 1 || phases[0].Failed != 3 {
		t.Fatalf("counts not folded: %+v", phases)
	}
}

// fakePermissionChecker grants the given permissions to user 1.
type fakePermissionChecker map[string]bool

func (f fakePermissionChecker) UserHasPermission(userID uint, permission string) bool {
	return userID == 1 && f[permission]
}

// newInsightsTestService builds a homepage service where user 1 has library:insights when
// role is admin.
func newInsightsTestService(t *testing.T, role string) *HomepageService {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	userRepo.EXPECT().GetByID(uint(1)).Return(&data.User{ID: 1, Role: role}, nil).AnyTimes()
	settings := NewSettingsService(nil, userRepo, zap.NewNop())
	svc := NewHomepageService(settings, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	svc.SetWidgetDeps(HomepageWidgetDeps{Permissions: fakePermissionChecker{PermLibraryInsights: role == "admin"}})
	return svc
}

func TestInsightsProcessingStatus_ForbiddenForNonAdmins(t *testing.T) {
	svc := newInsightsTestService(t, "user")
	section := data.HomepageSection{ID: "p", Type: "processing_status"}
	got, err := fetchInsightsProcessingWidget(svc, 1, section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	payload, ok := got.Widget.(InsightsProcessingPayload)
	if !ok || !payload.Forbidden || payload.Coverage != nil || len(payload.Phases) != 0 {
		t.Fatalf("non-admins must get nothing but forbidden: %+v", got.Widget)
	}
}

func TestInsightsWidgets_RequireDatabase(t *testing.T) {
	svc := newInsightsTestService(t, "admin")
	for _, fetch := range []func(*HomepageService, uint, data.HomepageSection) (*HomepageSectionData, error){
		fetchInsightsTrendsWidget, fetchInsightsPerformerSpotlightWidget, fetchInsightsCurationWidget, fetchInsightsProcessingWidget,
	} {
		if _, err := fetch(svc, 1, data.HomepageSection{Limit: 5, Config: map[string]interface{}{}}); err == nil {
			t.Fatal("expected an error without database")
		}
	}
}

func TestInsightsUserIsAdmin(t *testing.T) {
	if !insightsUserIsAdmin(newInsightsTestService(t, "admin"), 1) {
		t.Fatal("admin not recognized")
	}
	if insightsUserIsAdmin(newInsightsTestService(t, "moderator"), 1) {
		t.Fatal("moderator without library:insights must not get the insights")
	}
	if insightsUserIsAdmin(NewHomepageService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop()), 1) {
		t.Fatal("no permission checker: no insights")
	}
}
