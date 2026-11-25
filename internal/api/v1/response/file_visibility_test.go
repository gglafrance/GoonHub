package response

import (
	"encoding/json"
	"testing"
	"time"

	"goonhub/internal/core"
	"goonhub/internal/data"
)

func fileInfoScene() data.Scene {
	storagePathID := uint(2)
	fp := "audio"
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return data.Scene{
		ID: 9, Title: "Scene", Duration: 600, Width: 1920, Height: 1080,
		Size: 123456, StoredPath: "/media/a/scene.mp4", OriginalFilename: "scene.mp4", StoragePathID: &storagePathID,
		VideoCodec: "h264", AudioCodec: "aac", BitRate: 4000, FrameRate: 29.97, FileHash: "abc",
		FingerprintType: &fp, FingerprintAt: &at, ProcessingError: "boom",
		ThumbnailPath: "/thumbnails/9.jpg",
	}
}

func jsonKeys(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}
	return out
}

var allFileKeys = []string{
	"size", "stored_path", "original_filename", "storage_path_id",
	"video_codec", "audio_codec", "bit_rate", "frame_rate", "file_hash", "fingerprint_type", "fingerprint_at", "processing_error",
}

func TestFileVisibilityFor(t *testing.T) {
	if v := FileVisibilityFor(nil); v != (FileVisibility{}) {
		t.Fatalf("nil checker: %+v", v)
	}
	var none *core.UserAccess
	if v := FileVisibilityFor(none); v != (FileVisibility{}) {
		t.Fatalf("nil access: %+v", v)
	}
	if v := FileVisibilityFor(core.NewUserAccess(1, "admin", true)); v != FullFileVisibility {
		t.Fatalf("superuser: %+v", v)
	}
	v := FileVisibilityFor(core.NewUserAccess(1, "u", false, core.PermFilesViewSize))
	if !v.Size || v.Paths || v.Technical {
		t.Fatalf("size only: %+v", v)
	}
}

// A scene payload of a user without any files:view_* permission has no file key at all.
func TestSceneDetail_WithoutFilePermissions(t *testing.T) {
	scene := fileInfoScene()
	detail := NewSceneDetail(&scene, FileVisibility{})
	detail.Playback = map[string]string{"mode": "direct"}
	keys := jsonKeys(t, detail)
	for _, k := range allFileKeys {
		if _, ok := keys[k]; ok {
			t.Fatalf("key %q must be omitted, got %v", k, keys)
		}
	}
	for _, k := range []string{"id", "title", "duration", "width", "height", "thumbnail_path", "source_site", "playback"} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("key %q must stay, got %v", k, keys)
		}
	}
}

func TestSceneDetail_PerCategory(t *testing.T) {
	scene := fileInfoScene()
	tests := []struct {
		name    string
		vis     FileVisibility
		present []string
		absent  []string
	}{
		{"full", FullFileVisibility, allFileKeys, nil},
		{"size only", FileVisibility{Size: true}, []string{"size"}, []string{"stored_path", "original_filename", "storage_path_id", "video_codec", "frame_rate"}},
		{"paths only", FileVisibility{Paths: true}, []string{"stored_path", "original_filename", "storage_path_id"}, []string{"size", "file_hash", "bit_rate"}},
		{"technical only", FileVisibility{Technical: true}, []string{"video_codec", "audio_codec", "bit_rate", "frame_rate", "file_hash", "fingerprint_type", "fingerprint_at", "processing_error"}, []string{"size", "stored_path"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			keys := jsonKeys(t, NewSceneDetail(&scene, tc.vis))
			for _, k := range tc.present {
				if _, ok := keys[k]; !ok {
					t.Fatalf("key %q missing", k)
				}
			}
			for _, k := range tc.absent {
				if _, ok := keys[k]; ok {
					t.Fatalf("key %q must be omitted", k)
				}
			}
		})
	}
	if string(jsonKeys(t, NewSceneDetail(&scene, FullFileVisibility))["size"]) != "123456" {
		t.Fatal("size value changed")
	}
	if b, _ := json.Marshal(NewSceneDetail(nil, FullFileVisibility)); string(b) != "null" {
		t.Fatalf("nil scene: %s", b)
	}
}

func TestSceneListItem_Redaction(t *testing.T) {
	scene := fileInfoScene()
	fields := CardFields{FrameRate: true, Resolution: true}

	keys := jsonKeys(t, ToSceneListItemWithFields(scene, fields, FileVisibility{}))
	for _, k := range []string{"size", "stored_path", "frame_rate"} {
		if _, ok := keys[k]; ok {
			t.Fatalf("key %q must be omitted", k)
		}
	}
	for _, k := range []string{"width", "height", "duration"} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("key %q must stay", k)
		}
	}

	keys = jsonKeys(t, ToSceneListItemWithFields(scene, fields, FullFileVisibility))
	if string(keys["size"]) != "123456" || string(keys["stored_path"]) != `"/media/a/scene.mp4"` || string(keys["frame_rate"]) != "29.97" {
		t.Fatalf("full visibility: %v", keys)
	}
}

func TestRedactWidget(t *testing.T) {
	size := int64(42)
	stats := core.StatsWidgetPayload{SceneCount: 3, TotalSize: &size}
	records := core.RecordsPayload{BiggestFile: &core.RecordSceneRef{}, TotalViews: 4}

	if k := jsonKeys(t, redactWidget(stats, FileVisibility{})); k["total_size"] != nil || k["scene_count"] == nil {
		t.Fatalf("stats: %v", k)
	}
	if stats.TotalSize == nil {
		t.Fatal("the original payload must not be modified")
	}
	if k := jsonKeys(t, redactWidget(&records, FileVisibility{})); k["biggest_file"] != nil || k["total_views"] == nil {
		t.Fatalf("records: %v", k)
	}
	if records.BiggestFile == nil {
		t.Fatal("the original payload must not be modified")
	}
	if k := jsonKeys(t, redactWidget(stats, FileVisibility{Size: true})); string(k["total_size"]) != "42" {
		t.Fatalf("stats with size: %v", k)
	}
}

func TestFolderAndMatchInfoRedaction(t *testing.T) {
	folders := ToFolderInfos([]data.FolderInfo{{Name: "a", Path: "a", SceneCount: 2, TotalSize: 99}}, FileVisibility{})
	if folders[0].TotalSize != nil {
		t.Fatal("folder total size must be omitted")
	}
	name, size := "file.mp4", int64(5)
	resp := ToScenesMatchInfoResponse([]core.SceneMatchInfo{{ID: 1, OriginalFilename: &name, Size: &size}}, FileVisibility{Size: true})
	if resp.Scenes[0].OriginalFilename != nil || resp.Scenes[0].Size == nil {
		t.Fatalf("match info: %+v", resp.Scenes[0])
	}
}
