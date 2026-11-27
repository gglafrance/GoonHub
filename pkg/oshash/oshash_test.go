package oshash

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// pattern returns n bytes where byte i is i % 251 (no period aligned on 8 bytes, so the head
// and the tail differ).
func pattern(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

func writeFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// The expected values were computed independently (Python, struct.unpack('<Q') sums of the
// head and tail plus the size, as Stash's pkg/hash/oshash).
func TestFromFileKnownValues(t *testing.T) {
	tests := []struct {
		name string
		size int
		want string
	}{
		{"larger than two chunks", 200 * 1024, "a65c16d28845ff55"},
		{"exactly one chunk", 64 * 1024, "d48b4000b76d0d86"},
		{"smaller than a chunk, head and tail overlap", 1003, "848b868b928d9706"},
		{"smaller than one word", 5, "0000000000000005"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromFile(writeFile(t, "v.mp4", pattern(tt.size)))
			if err != nil {
				t.Fatalf("FromFile: %v", err)
			}
			if got != tt.want {
				t.Fatalf("oshash = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFromFileSameContentSameHash(t *testing.T) {
	content := pattern(300 * 1024)
	a, err := FromFile(writeFile(t, "a.mp4", content))
	if err != nil {
		t.Fatal(err)
	}
	b, err := FromFile(writeFile(t, "other-name.mkv", content))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("identical files hash differently: %s vs %s", a, b)
	}
	// Same size, one byte changed in the tail
	changed := bytes.Clone(content)
	changed[len(changed)-10] ^= 0xff
	c, err := FromFile(writeFile(t, "c.mp4", changed))
	if err != nil {
		t.Fatal(err)
	}
	if c == a {
		t.Fatal("a file with a different tail must hash differently")
	}
}

func TestFromFileEmptyAndMissing(t *testing.T) {
	if _, err := FromFile(writeFile(t, "empty.mp4", nil)); !errors.Is(err, ErrEmptyFile) {
		t.Fatalf("empty file: err = %v, want ErrEmptyFile", err)
	}
	if _, err := FromFile(filepath.Join(t.TempDir(), "missing.mp4")); err == nil {
		t.Fatal("missing file must fail")
	}
	if _, err := FromFile(t.TempDir()); err == nil {
		t.Fatal("a directory must fail")
	}
}

// The file is opened read-only and left untouched (content and modification time).
func TestFromFileOpensReadOnly(t *testing.T) {
	content := pattern(150 * 1024)
	p := writeFile(t, "ro.mp4", content)
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}

	var flags []int
	orig := openFile
	openFile = func(name string, flag int, perm fs.FileMode) (*os.File, error) {
		flags = append(flags, flag)
		return orig(name, flag, perm)
	}
	t.Cleanup(func() { openFile = orig })

	if _, err := FromFile(p); err != nil {
		t.Fatalf("FromFile on a read-only file: %v", err)
	}
	if len(flags) != 1 || flags[0] != os.O_RDONLY {
		t.Fatalf("open flags = %v, want [O_RDONLY]", flags)
	}
	got, err := os.ReadFile(p)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("file content changed (%v)", err)
	}
	info, err := os.Stat(p)
	if err != nil || !info.ModTime().Equal(old) {
		t.Fatalf("modification time changed: %v (%v)", info.ModTime(), err)
	}
}
