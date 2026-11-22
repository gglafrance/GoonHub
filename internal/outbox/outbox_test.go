package outbox

import (
	"os"
	"path/filepath"
	"testing"

	"goonhub/pkg/workerproto"
)

func assignment(id string, gen int64) workerproto.Assignment {
	return workerproto.Assignment{JobID: id, Phase: workerproto.PhaseSprites, SceneID: 1, Lease: workerproto.Lease{Generation: gen}}
}

func TestCreateLoadRemove(t *testing.T) {
	ob, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e, err := ob.Create(assignment("5b0c-aa", 3))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.ArtifactPath("1_thumbnails.vtt"), []byte("WEBVTT"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.State.Stage = StageComputed
	e.State.Complete = &workerproto.CompleteRequest{Generation: 3, Phase: workerproto.PhaseSprites,
		Artifacts: []workerproto.ArtifactRef{{Name: "1_thumbnails.vtt", Size: 6}}}
	if err := e.Save(); err != nil {
		t.Fatal(err)
	}

	got, errs := ob.Load()
	if len(errs) != 0 || len(got) != 1 {
		t.Fatalf("load: %v %v", got, errs)
	}
	g := got[0]
	if g.Assignment.JobID != "5b0c-aa" || g.State.Stage != StageComputed || g.State.Generation != 3 || g.State.Complete == nil {
		t.Fatalf("reloaded %+v", g)
	}
	if !g.State.Stage.Resumable() || StageComputing.Resumable() {
		t.Fatal("Resumable")
	}
	if err := ob.Remove(g); err != nil {
		t.Fatal(err)
	}
	if got, _ := ob.Load(); len(got) != 0 {
		t.Fatalf("not removed: %v", got)
	}
}

func TestLoadDropsHalfCreatedEntries(t *testing.T) {
	root := t.TempDir()
	ob, _ := Open(root)
	// A crash between mkdir and assignment.json, and a torn assignment.json.
	if err := os.MkdirAll(filepath.Join(root, "half", "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "torn"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "torn", "assignment.json"), []byte(`{"job_id": "x`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Assignment written, state missing: computing.
	e, err := ob.Create(assignment("ok", 2))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(e.Dir, "state.json")); err != nil {
		t.Fatal(err)
	}
	got, errs := ob.Load()
	if len(errs) != 0 || len(got) != 1 || got[0].State.Stage != StageComputing || got[0].State.Generation != 2 {
		t.Fatalf("load: %+v %v", got, errs)
	}
	for _, d := range []string{"half", "torn"} {
		if _, err := os.Stat(filepath.Join(root, d)); !os.IsNotExist(err) {
			t.Fatalf("%s not cleaned", d)
		}
	}
}

func TestDirName(t *testing.T) {
	if DirName("5b0c-1234_ab") != "5b0c-1234_ab" {
		t.Fatal("safe id changed")
	}
	for _, bad := range []string{"../x", "a/b", `c:\x`, ""} {
		n := DirName(bad)
		if len(n) != 34 || n[:2] != "j-" {
			t.Fatalf("DirName(%q) = %q", bad, n)
		}
	}
}

func TestWriteFileAtomicReplaces(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	for _, v := range []string{"one", "two"} {
		if err := WriteFileAtomic(p, []byte(v)); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(p)
	if string(b) != "two" {
		t.Fatalf("content %q", b)
	}
	items, _ := os.ReadDir(filepath.Dir(p))
	if len(items) != 1 {
		t.Fatalf("temp files left: %v", items)
	}
}
