package workerproto

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateArtifactName(t *testing.T) {
	valid := []string{"12_sheet_001.webp", "12_thumbnails.vtt", "marker_7.mp4", "3_preview.mp4", "a", "A-b_c.d"}
	for _, name := range valid {
		if err := ValidateArtifactName(name); err != nil {
			t.Fatalf("ValidateArtifactName(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{
		"", ".", "..", "../x.mp4", "..\\x.mp4", "a/b.mp4", "a\\b.mp4", "/etc/passwd", "C:x.mp4",
		".hidden", "trailing.", "a..b", "with space.mp4", "nul", "CON.txt", "com1.mp4", "é.mp4",
		"line\nbreak", strings.Repeat("a", MaxArtifactNameLen+1),
	}
	for _, name := range invalid {
		if err := ValidateArtifactName(name); !errors.Is(err, ErrInvalidArtifactName) {
			t.Fatalf("ValidateArtifactName(%q) = %v, want ErrInvalidArtifactName", name, err)
		}
	}
	if err := ValidateArtifactName(strings.Repeat("a", MaxArtifactNameLen)); err != nil {
		t.Fatalf("a name of the maximum length must pass: %v", err)
	}
}

func TestSHA256File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.bin")
	if err := os.WriteFile(path, []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}
	sum, size, err := SHA256File(path)
	if err != nil {
		t.Fatalf("SHA256File: %v", err)
	}
	if sum != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" || size != 3 {
		t.Fatalf("sum = %s size = %d", sum, size)
	}
	if !ValidSHA256(sum) || ValidSHA256(strings.ToUpper(sum)) || ValidSHA256(sum[:10]) {
		t.Fatal("ValidSHA256 wrong")
	}
	ref, err := ArtifactRefForFile("f.bin", path)
	if err != nil || ref.SHA256 != sum || ref.Size != 3 || ref.Name != "f.bin" {
		t.Fatalf("ArtifactRefForFile = %+v, %v", ref, err)
	}
	if _, err := ArtifactRefForFile("../f.bin", path); err == nil {
		t.Fatal("ArtifactRefForFile must validate the name")
	}
	if _, _, err := SHA256File(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing file must fail")
	}
}

func TestValidateArtifacts(t *testing.T) {
	sum := strings.Repeat("a", 64)
	result := PhaseResult{Sprites: &SpritesResult{SpriteSheets: []string{"1_sheet_001.webp"}, VTT: "1_thumbnails.vtt", SheetCount: 1}}
	refs := []ArtifactRef{{Name: "1_sheet_001.webp", Size: 10, SHA256: sum}, {Name: "1_thumbnails.vtt", Size: 5, SHA256: sum}}
	if err := ValidateArtifacts(refs, result); err != nil {
		t.Fatalf("ValidateArtifacts: %v", err)
	}
	if err := ValidateArtifacts(refs[:1], result); err == nil {
		t.Fatal("an undeclared referenced file must fail")
	}
	if err := ValidateArtifacts(append(refs, refs[0]), result); err == nil {
		t.Fatal("a duplicate artifact must fail")
	}
	if err := ValidateArtifacts([]ArtifactRef{{Name: "x", SHA256: "nope"}}, PhaseResult{}); err == nil {
		t.Fatal("a malformed sha256 must fail")
	}
}

func TestPhaseParamsAndResultValidate(t *testing.T) {
	p := PhaseParams{Sprites: &SpritesParams{}}
	if err := p.Validate(PhaseSprites); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := p.Validate(PhaseFingerprint); err == nil {
		t.Fatal("params of another phase must fail")
	}
	both := PhaseParams{Sprites: &SpritesParams{}, Fingerprint: &FingerprintParams{}}
	if err := both.Validate(PhaseSprites); err == nil {
		t.Fatal("two params set must fail")
	}
	r := PhaseResult{Fingerprint: &FingerprintResult{}}
	if err := r.Validate(PhaseFingerprint); err != nil || r.Validate(PhaseSprites) == nil {
		t.Fatal("PhaseResult.Validate wrong")
	}
	for _, ph := range RemotePhases {
		if !ph.Valid() {
			t.Fatalf("%s must be valid", ph)
		}
	}
	if Phase("metadata").Valid() || Phase("thumbnail").Valid() {
		t.Fatal("metadata/thumbnail are never remote")
	}
}

func TestSpritesResultValidate(t *testing.T) {
	ok := &SpritesResult{SpriteSheets: []string{"5_sheet_001.webp", "5_sheet_002.webp"}, VTT: "5_thumbnails.vtt", SheetCount: 2}
	if err := ok.Validate(5); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	bad := []*SpritesResult{
		{SpriteSheets: []string{"6_sheet_001.webp"}, VTT: "5_thumbnails.vtt", SheetCount: 1},
		{SpriteSheets: []string{"5_sheet_002.webp"}, VTT: "5_thumbnails.vtt", SheetCount: 1},
		{SpriteSheets: []string{"5_sheet_001.webp"}, VTT: "6_thumbnails.vtt", SheetCount: 1},
		{SpriteSheets: []string{"5_sheet_001.webp"}, VTT: "5_thumbnails.vtt", SheetCount: 2},
		{SpriteSheets: []string{"../5_sheet_001.webp"}, VTT: "5_thumbnails.vtt", SheetCount: 1},
	}
	for i, r := range bad {
		if err := r.Validate(5); err == nil {
			t.Fatalf("bad result %d must fail", i)
		}
	}
	if err := (&SpritesResult{SpriteSheets: []string{}, VTT: "5_thumbnails.vtt"}).Validate(5); err != nil {
		t.Fatalf("no sheet (too short video) is valid: %v", err)
	}
}

func TestAnimatedThumbnailsValidate(t *testing.T) {
	p := &AnimatedThumbnailsParams{
		SceneID: 9,
		Markers: []MarkerClipSpec{{MarkerID: 1, DurationSec: 10, Output: "marker_1.mp4"}},
		Preview: &ScenePreviewSpec{Output: "9_preview.mp4", Segments: 12, SegmentDuration: 1, Width: 360, CRF: 27},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := (&AnimatedThumbnailsParams{SceneID: 9, Markers: []MarkerClipSpec{{MarkerID: 1, Output: "marker_2.mp4"}}}).Validate(); err == nil {
		t.Fatal("a wrong marker output name must fail")
	}
	if err := (&AnimatedThumbnailsParams{SceneID: 9, Preview: &ScenePreviewSpec{Output: "8_preview.mp4"}}).Validate(); err == nil {
		t.Fatal("a wrong preview name must fail")
	}

	good := &AnimatedThumbnailsResult{Markers: []MarkerClipResult{{MarkerID: 1, File: "marker_1.mp4"}}, Preview: &ScenePreviewResult{File: "9_preview.mp4"}}
	if err := good.Validate(p); err != nil {
		t.Fatalf("result Validate: %v", err)
	}
	for i, r := range []*AnimatedThumbnailsResult{
		{Markers: []MarkerClipResult{{MarkerID: 2, File: "marker_2.mp4"}}},
		{Markers: []MarkerClipResult{{MarkerID: 1, File: "marker_9.mp4"}}},
		{Markers: []MarkerClipResult{{MarkerID: 1, File: "marker_1.mp4"}, {MarkerID: 1, File: "marker_1.mp4"}}},
		{Preview: &ScenePreviewResult{File: "x.mp4"}},
	} {
		if err := r.Validate(p); err == nil {
			t.Fatalf("bad result %d must fail", i)
		}
	}
	if err := (&AnimatedThumbnailsResult{Preview: &ScenePreviewResult{File: "9_preview.mp4"}}).Validate(&AnimatedThumbnailsParams{SceneID: 9}); err == nil {
		t.Fatal("an unasked preview must fail")
	}
	files := PhaseResult{AnimatedThumbnails: good}.Files()
	if strings.Join(files, ",") != "marker_1.mp4,9_preview.mp4" {
		t.Fatalf("Files = %v", files)
	}
}

func TestFingerprintJSONRoundTrip(t *testing.T) {
	in := FingerprintResult{
		AudioFingerprint:  AudioFingerprint{-1, 0, 2147483647, -2147483648},
		VisualFingerprint: VisualFingerprint{0, 1, 0xffffffffffffffff},
		AudioDurationSec:  12.5,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"audio_fingerprint":"`) {
		t.Fatalf("fingerprints must be base64 strings: %s", b)
	}
	var out FingerprintResult
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(out.AudioFingerprint) != 4 || out.AudioFingerprint[3] != -2147483648 || out.VisualFingerprint[2] != 0xffffffffffffffff {
		t.Fatalf("round trip = %+v", out)
	}
	if out.FingerprintTypeLabel() != "dual" {
		t.Fatalf("label = %s", out.FingerprintTypeLabel())
	}

	// Empty fingerprints are omitted and decode as nil
	b, _ = json.Marshal(FingerprintResult{VisualFingerprint: VisualFingerprint{7}})
	if strings.Contains(string(b), "audio_fingerprint") {
		t.Fatalf("empty audio must be omitted: %s", b)
	}
	var vis FingerprintResult
	if err := json.Unmarshal(b, &vis); err != nil || vis.AudioFingerprint != nil || vis.FingerprintTypeLabel() != "visual" {
		t.Fatalf("visual only = %+v, %v", vis, err)
	}

	// Storage format: little-endian
	if got := (AudioFingerprint{1}).Bytes(); len(got) != 4 || got[0] != 1 {
		t.Fatalf("Bytes = %v", got)
	}
	if _, err := AudioFingerprintFromBytes([]byte{1, 2, 3}); err == nil {
		t.Fatal("a length not multiple of 4 must fail")
	}
	if _, err := VisualFingerprintFromBytes([]byte{1}); err == nil {
		t.Fatal("a length not multiple of 8 must fail")
	}
	if err := json.Unmarshal([]byte(`{"audio_fingerprint":"!!"}`), &out); err == nil {
		t.Fatal("invalid base64 must fail")
	}
	if (&FingerprintResult{}).Validate() == nil {
		t.Fatal("an empty fingerprint result must not validate")
	}
}

func TestEffectiveSpriteInterval(t *testing.T) {
	tests := []struct {
		name                          string
		duration, interval, minFrames int
		want                          int
	}{
		{"long scene keeps interval", 3600, 10, 12, 10},
		{"exactly enough frames", 120, 10, 12, 10},
		{"short scene gets denser frames", 60, 10, 12, 5},
		{"very short scene uses 1s", 7, 10, 12, 1},
		{"unknown duration keeps interval", 0, 10, 12, 10},
		{"disabled interval untouched", 30, 0, 12, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveSpriteInterval(tt.duration, tt.interval, tt.minFrames); got != tt.want {
				t.Fatalf("EffectiveSpriteInterval(%d, %d, %d) = %d, want %d", tt.duration, tt.interval, tt.minFrames, got, tt.want)
			}
		})
	}
}

func TestMediaSignature(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Unix(1_800_000_000, 0)
	exp := now.Add(time.Hour)
	raw := MediaURL("http://server:8080", secret, "job-1", 3, exp)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Path != "/api/v1/workers/media/job-1" {
		t.Fatalf("path = %s", u.Path)
	}
	gen, err := VerifyMedia(secret, "job-1", u.Query(), now)
	if err != nil || gen != 3 {
		t.Fatalf("VerifyMedia = %d, %v", gen, err)
	}
	if _, err := VerifyMedia(secret, "job-2", u.Query(), now); err == nil {
		t.Fatal("a URL of another job must fail")
	}
	if _, err := VerifyMedia([]byte("other"), "job-1", u.Query(), now); err == nil {
		t.Fatal("another secret must fail")
	}
	if _, err := VerifyMedia(secret, "job-1", u.Query(), exp.Add(time.Second)); err == nil {
		t.Fatal("an expired URL must fail")
	}
	q := u.Query()
	q.Set(MediaParamGeneration, "4")
	if _, err := VerifyMedia(secret, "job-1", q, now); err == nil {
		t.Fatal("a tampered generation must fail")
	}
}

func TestPaths(t *testing.T) {
	if got := JobPath(PathJobComplete, "a b"); got != "/api/v1/workers/jobs/a%20b/complete" {
		t.Fatalf("JobPath = %s", got)
	}
	if got := ArtifactPath("j1", "1_sheet_001.webp"); got != "/api/v1/workers/jobs/j1/artifacts/1_sheet_001.webp" {
		t.Fatalf("ArtifactPath = %s", got)
	}
	if !ReleaseCrashRecovery.CountsAsAttempt() || ReleaseShutdown.CountsAsAttempt() {
		t.Fatal("CountsAsAttempt wrong")
	}
}

func TestAssignmentJSONShape(t *testing.T) {
	a := Assignment{
		JobID: "j", Phase: PhaseSprites, SceneID: 1,
		Lease:  Lease{Generation: 2, DurationSec: 60},
		Input:  InputSpec{ServerPath: "/m/a.mp4", URL: "http://s/x", Video: VideoInfo{DurationSec: 10}},
		Params: PhaseParams{Sprites: &SpritesParams{SceneID: 1}},
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"job_id":"j"`, `"generation":2`, `"server_path":"/m/a.mp4"`, `"sprites":{`, `"duration_sec":10`} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("JSON missing %s: %s", key, b)
		}
	}
	if strings.Contains(string(b), `"fingerprint"`) {
		t.Fatalf("unset params must be omitted: %s", b)
	}
}
