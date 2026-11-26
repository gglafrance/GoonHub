package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
)

// fakeShareMarkers stands in for the marker reads of the share links.
type fakeShareMarkers struct {
	rows    map[uint]data.ShareMarkerRow
	visible map[uint]bool
}

func (f fakeShareMarkers) MarkerVisibleTo(_ context.Context, markerID, _ uint) (bool, error) {
	return f.visible[markerID], nil
}

func (f fakeShareMarkers) MarkerCards(_ context.Context, ids []uint) ([]data.ShareMarkerRow, error) {
	var out []data.ShareMarkerRow
	for _, id := range ids {
		if r, ok := f.rows[id]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func TestShareLinkOptions(t *testing.T) {
	intp := func(v int) *int { return &v }
	uintp := func(v uint) *uint { return &v }
	end := 50
	markers := fakeShareMarkers{
		rows: map[uint]data.ShareMarkerRow{
			1: {ID: 1, SceneID: 5, Timestamp: 10, EndTimestamp: &end, Label: "Kiss"},
			2: {ID: 2, SceneID: 5, Timestamp: 100},                 // no end: default length
			3: {ID: 3, SceneID: 5, Timestamp: 110, Label: "Late"},  // the default length passes the end
			4: {ID: 4, SceneID: 6, Timestamp: 1},                   // another scene
			5: {ID: 5, SceneID: 5, Timestamp: 1, Label: "Private"}, // not visible to the user
		},
		visible: map[uint]bool{1: true, 2: true, 3: true, 4: true},
	}
	tests := []struct {
		name       string
		opts       ShareLinkOptions
		wantErr    func(error) bool
		start, end *int
		label      string
		markerRef  bool
	}{
		{name: "no option", opts: ShareLinkOptions{}},
		{name: "start", opts: ShareLinkOptions{Start: intp(42)}, start: intp(42)},
		{name: "start 0 is the beginning", opts: ShareLinkOptions{Start: intp(0)}},
		{name: "negative start", opts: ShareLinkOptions{Start: intp(-1)}, wantErr: apperrors.IsValidation},
		{name: "start past the end", opts: ShareLinkOptions{Start: intp(120)}, wantErr: apperrors.IsValidation},
		{name: "marker range", opts: ShareLinkOptions{MarkerID: uintp(1), Start: intp(99)}, start: intp(10), end: intp(50), label: "Kiss", markerRef: true},
		{name: "marker without end", opts: ShareLinkOptions{MarkerID: uintp(2)}, start: intp(100), end: intp(115), markerRef: true},
		{name: "marker cut at the scene end", opts: ShareLinkOptions{MarkerID: uintp(3)}, start: intp(110), end: intp(120), label: "Late", markerRef: true},
		{name: "marker of another scene", opts: ShareLinkOptions{MarkerID: uintp(4)}, wantErr: apperrors.IsValidation},
		{name: "marker not visible", opts: ShareLinkOptions{MarkerID: uintp(5)}, wantErr: apperrors.IsNotFound},
		{name: "unknown marker", opts: ShareLinkOptions{MarkerID: uintp(9)}, wantErr: apperrors.IsNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, shareLinkRepo, sceneRepo := newTestShareService(t)
			svc.SetMarkers(markers, 15)
			sceneRepo.EXPECT().GetByID(uint(5)).Return(&data.Scene{ID: 5, Duration: 120}, nil)
			var saved *data.ShareLink
			if tc.wantErr == nil {
				shareLinkRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(l *data.ShareLink) error {
					saved = l
					return nil
				})
			}
			_, err := svc.CreateShareLinkWithOptions(10, 5, data.ShareTypePublic, "7d", tc.opts)
			if tc.wantErr != nil {
				if err == nil || !tc.wantErr(err) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			eq := func(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
			if !eq(saved.Start, tc.start) || !eq(saved.End, tc.end) {
				t.Fatalf("range = %v-%v, want %v-%v", saved.Start, saved.End, tc.start, tc.end)
			}
			if got := ""; saved.Label != nil {
				got = *saved.Label
				if got != tc.label {
					t.Fatalf("label = %q, want %q", got, tc.label)
				}
			} else if tc.label != "" {
				t.Fatalf("no label, want %q", tc.label)
			}
			if (saved.MarkerID != nil) != tc.markerRef {
				t.Fatalf("marker ref = %v", saved.MarkerID)
			}
		})
	}
}

func TestShareLinkJSONExposesTheRangeOnly(t *testing.T) {
	start, end := 10, 40
	label := "Kiss"
	marker := uint(3)
	b, err := json.Marshal(ResolvedShareLink{ShareLink: data.ShareLink{Token: "t", Start: &start, End: &end, Label: &label, MarkerID: &marker}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"start":10`, `"end":40`, `"label":"Kiss"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("%s missing in %s", want, s)
		}
	}
	if strings.Contains(s, "marker") {
		t.Fatalf("the marker reference leaks: %s", s)
	}
}
