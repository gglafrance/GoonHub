package data

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestSceneListView_ScanNilGivesDefaults(t *testing.T) {
	var v SceneListView
	if err := v.Scan(nil); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if v.Pages == nil || len(v.Pages) != 0 || !slices.Equal(v.Columns, DefaultSceneListViewColumns()) {
		t.Fatalf("unexpected defaults: %+v", v)
	}
}

func TestSceneListView_ScanFillsMissingParts(t *testing.T) {
	v := SceneListView{Pages: map[string]string{"actor": "table"}, Columns: []string{"title"}}
	// A reused value must not keep its previous pages
	if err := v.Scan([]byte(`{"pages": {"search": "table"}}`)); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(v.Pages) != 1 || v.Pages["search"] != "table" {
		t.Fatalf("pages = %v", v.Pages)
	}
	if !slices.Equal(v.Columns, DefaultSceneListViewColumns()) {
		t.Fatalf("columns = %v", v.Columns)
	}
}

func TestSceneListView_ScanRejectsNonBytes(t *testing.T) {
	var v SceneListView
	if err := v.Scan("not bytes"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestSceneListView_ValueRoundTrip(t *testing.T) {
	// The zero value (a settings row created by another endpoint) is stored with its defaults
	raw, err := SceneListView{}.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	var got SceneListView
	if err := json.Unmarshal(raw.([]byte), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Pages == nil || !slices.Equal(got.Columns, DefaultSceneListViewColumns()) {
		t.Fatalf("stored zero value = %s", raw)
	}

	in := SceneListView{Pages: map[string]string{"explorer": "table"}, Columns: []string{"thumbnail", "title"}}
	raw, err = in.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	var out SceneListView
	if err := out.Scan(raw); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if out.Pages["explorer"] != "table" || !slices.Equal(out.Columns, in.Columns) {
		t.Fatalf("round trip = %+v", out)
	}
}
