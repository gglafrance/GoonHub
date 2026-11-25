package response

import (
	"encoding/json"

	"goonhub/internal/core"
	"goonhub/internal/data"
)

// FileVisibility tells which file information a user may see (docs/RBAC.md, "Information
// shown per permission"). The scene payloads omit the fields of a hidden category.
type FileVisibility struct {
	// Size: files:view_size (size, totals, biggest file)
	Size bool
	// Paths: files:view_paths (stored_path, original_filename, storage_path_id)
	Paths bool
	// Technical: files:view_technical (codecs, bit rate, frame rate, hash, fingerprint,
	// processing error)
	Technical bool
}

// FullFileVisibility shows everything (admin-only payloads, internal callers).
var FullFileVisibility = FileVisibility{Size: true, Paths: true, Technical: true}

// PermissionChecker is anything that can tell whether a permission is granted
// (*core.UserAccess).
type PermissionChecker interface {
	Has(permission string) bool
}

// FileVisibilityFor builds the visibility of a user from their permissions (nil: nothing).
func FileVisibilityFor(p PermissionChecker) FileVisibility {
	if p == nil {
		return FileVisibility{}
	}
	return FileVisibility{
		Size:      p.Has(core.PermFilesViewSize),
		Paths:     p.Has(core.PermFilesViewPaths),
		Technical: p.Has(core.PermFilesViewTech),
	}
}

// sceneSizeKeys, scenePathKeys and sceneTechnicalKeys are the JSON keys of data.Scene in
// each category.
var (
	sceneSizeKeys      = []string{"size"}
	scenePathKeys      = []string{"stored_path", "original_filename", "storage_path_id"}
	sceneTechnicalKeys = []string{"video_codec", "audio_codec", "bit_rate", "frame_rate", "file_hash", "fingerprint_type", "fingerprint_at", "processing_error"}
)

// hiddenSceneKeys returns the data.Scene JSON keys the visibility hides.
func (v FileVisibility) hiddenSceneKeys() []string {
	var keys []string
	if !v.Size {
		keys = append(keys, sceneSizeKeys...)
	}
	if !v.Paths {
		keys = append(keys, scenePathKeys...)
	}
	if !v.Technical {
		keys = append(keys, sceneTechnicalKeys...)
	}
	return keys
}

// int64If returns a pointer to n when shown, nil otherwise (omitted from the JSON).
func int64If(show bool, n int64) *int64 {
	if !show {
		return nil
	}
	return &n
}

// stringIf returns a pointer to s when shown, nil otherwise (omitted from the JSON).
func stringIf(show bool, s string) *string {
	if !show {
		return nil
	}
	return &s
}

// marshalSceneWithout marshals a scene with extra keys, without the keys the visibility
// hides.
func marshalSceneWithout(scene *data.Scene, vis FileVisibility, extra map[string]any) ([]byte, error) {
	raw, err := json.Marshal(scene)
	if err != nil {
		return nil, err
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for _, k := range vis.hiddenSceneKeys() {
		delete(fields, k)
	}
	for k, v := range extra {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		fields[k] = b
	}
	return json.Marshal(fields)
}
