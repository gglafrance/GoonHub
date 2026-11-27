package together

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/together/togethertest"

	"go.uber.org/zap"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gifBytes(t *testing.T, frames int) []byte {
	t.Helper()
	g := &gif.GIF{}
	for i := 0; i < frames; i++ {
		g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, 24, 16), palette.Plan9))
		g.Delay = append(g.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// webpVP8X builds the header of an extended WebP (canvas size and animation flag).
func webpVP8X(w, h int, animated bool) []byte {
	b := []byte("RIFF\x00\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00")
	flags := byte(0)
	if animated {
		flags = 0x02
	}
	b = append(b, flags, 0, 0, 0)
	b = append(b, byte(w-1), byte((w-1)>>8), byte((w-1)>>16), byte(h-1), byte((h-1)>>8), byte((h-1)>>16))
	return append(b, make([]byte, 16)...)
}

func webpVP8L(w, h int) []byte {
	b := []byte("RIFF\x00\x00\x00\x00WEBPVP8L\x05\x00\x00\x00\x2f")
	bits := uint32(w-1) | uint32(h-1)<<14
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], bits)
	b = append(b, tmp[:]...)
	return append(b, make([]byte, 16)...)
}

func TestInspectEmote(t *testing.T) {
	cases := []struct {
		name     string
		b        []byte
		ext      string
		w, h     int
		animated bool
	}{
		{"png", pngBytes(t, 32, 28), "png", 32, 28, false},
		{"gif still", gifBytes(t, 1), "gif", 24, 16, false},
		{"gif animated", gifBytes(t, 3), "gif", 24, 16, true},
		{"webp extended animated", webpVP8X(112, 64, true), "webp", 112, 64, true},
		{"webp extended still", webpVP8X(300, 2, false), "webp", 300, 2, false},
		{"webp lossless", webpVP8L(48, 40), "webp", 48, 40, false},
	}
	for _, c := range cases {
		img, err := inspectEmote(c.b)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if img.ext != c.ext || img.width != c.w || img.height != c.h || img.animated != c.animated {
			t.Fatalf("%s: got %+v", c.name, img)
		}
	}
	for _, bad := range [][]byte{[]byte("<svg></svg>"), []byte("GIF89a broken"), []byte("RIFF\x00\x00\x00\x00WEBPXXXX")} {
		if _, err := inspectEmote(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func newEmoteService(t *testing.T) (*EmoteService, string, *int) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "emotes")
	cfg := config.Goon2getherConfig{EmoteDir: dir, EmoteMaxBytes: 64 << 10, EmoteMaxDimension: 128, EmoteMaxCount: 3}
	s := NewEmoteService(togethertest.NewEmotes(), cfg, zap.NewNop())
	changes := 0
	s.OnChange(func() { changes++ })
	return s, dir, &changes
}

func TestEmoteUploadRenameDelete(t *testing.T) {
	s, dir, changes := newEmoteService(t)

	e, err := s.Upload("PogChamp", bytes.NewReader(pngBytes(t, 64, 64)), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(e.URL, "/emotes/") || e.Width != 64 || e.Animated {
		t.Fatalf("emote %+v", e)
	}
	file := strings.TrimPrefix(e.URL, "/emotes/")
	if !EmoteFilePattern.MatchString(file) {
		t.Fatalf("stored name %q", file)
	}
	if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
		t.Fatalf("file not stored: %v", err)
	}
	if !s.HasEmote("pogchamp") || s.HasEmote("other") {
		t.Fatal("HasEmote is wrong")
	}

	// Refusals
	for name, tc := range map[string]struct {
		name string
		b    []byte
	}{
		"bad name":      {"a", pngBytes(t, 8, 8)},
		"name chars":    {"no-dash", pngBytes(t, 8, 8)},
		"too large":     {"big", pngBytes(t, 129, 10)},
		"not an image":  {"svg", []byte("<svg/>")},
		"too heavy":     {"heavy", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 70<<10)...)},
		"name taken ci": {"pogchamp", pngBytes(t, 8, 8)},
	} {
		_, err := s.Upload(tc.name, bytes.NewReader(tc.b), 1)
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
		if name == "name taken ci" && !apperrors.IsConflict(err) {
			t.Fatalf("%s: %v", name, err)
		}
		if name != "name taken ci" && !apperrors.IsValidation(err) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("refused uploads left files: %d", len(entries))
	}

	renamed, err := s.Rename(e.ID, "Kappa")
	if err != nil || renamed.Name != "Kappa" || renamed.URL != e.URL {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	if !s.HasEmote("kappa") || s.HasEmote("pogchamp") {
		t.Fatal("the name cache was not refreshed")
	}
	if _, err := s.Rename(999, "Other"); !apperrors.IsNotFound(err) {
		t.Fatalf("rename unknown: %v", err)
	}

	// The count limit
	for _, n := range []string{"two", "three"} {
		if _, err := s.Upload(n, bytes.NewReader(pngBytes(t, 8, 8)), 1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Upload("four", bytes.NewReader(pngBytes(t, 8, 8)), 1); !apperrors.IsValidation(err) {
		t.Fatalf("count limit: %v", err)
	}

	if err := s.Delete(e.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, file)); !os.IsNotExist(err) {
		t.Fatalf("the image file was not removed: %v", err)
	}
	if err := s.Delete(e.ID); !apperrors.IsNotFound(err) {
		t.Fatalf("delete twice: %v", err)
	}
	list, _ := s.List()
	if len(list) != 2 {
		t.Fatalf("list %+v", list)
	}
	if *changes != 5 {
		t.Fatalf("expected 5 change notifications, got %d", *changes)
	}
}
