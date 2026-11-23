// Package outbox keeps the worker's jobs on disk until the server acknowledged them, so a
// crash, a kill or a network loss loses nothing (docs/REMOTE_WORKERS.md, section 11).
//
// One directory per job: <root>/<job>/assignment.json (written first, fsync), state.json
// (stage, generation, the CompleteRequest once computed) and out/ (the artifacts). Every
// JSON write is a temp file + fsync + rename.
package outbox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"goonhub/pkg/workerproto"
)

// Stage is the persisted progress of a job.
type Stage string

const (
	// StageComputing: computation started; the output is partial (not resumable).
	StageComputing Stage = "computing"
	// StageComputed: the result and the artifact list are final, nothing uploaded yet.
	StageComputed Stage = "computed"
	// StageUploading: some artifacts may be on the server already.
	StageUploading Stage = "uploading"
	// StageCompleting: every artifact uploaded, complete in flight.
	StageCompleting Stage = "completing"
)

// Resumable reports whether the stage holds a final result (upload/complete can resume).
func (s Stage) Resumable() bool {
	return s == StageComputed || s == StageUploading || s == StageCompleting
}

// State is state.json.
type State struct {
	Stage      Stage                 `json:"stage"`
	Generation int64                 `json:"generation"`
	InputMode  workerproto.InputMode `json:"input_mode,omitempty"`
	// Complete is the request to send once every artifact is uploaded (set from computed).
	Complete *workerproto.CompleteRequest `json:"complete,omitempty"`
	// Uploaded are the last known upload offsets (informational: the server's GET status
	// is the truth).
	Uploaded map[string]int64 `json:"uploaded,omitempty"`
	// ReleasePending is set when a graceful stop could not reach the server to release the
	// job: the next start releases it with this reason.
	ReleasePending workerproto.ReleaseReason `json:"release_pending,omitempty"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

// Outbox is the root directory of the job entries.
type Outbox struct {
	root string
}

// Entry is one job on disk.
type Entry struct {
	Dir        string
	Assignment workerproto.Assignment
	State      State
}

const (
	assignmentFile = "assignment.json"
	stateFile      = "state.json"
	outDir         = "out"
	tmpDir         = "tmp"
)

// Open creates the root if needed.
func Open(root string) (*Outbox, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create outbox %s: %w", root, err)
	}
	return &Outbox{root: root}, nil
}

// Root returns the outbox directory.
func (o *Outbox) Root() string { return o.root }

var safeID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

// DirName maps a job id to a safe directory name.
func DirName(jobID string) string {
	if safeID.MatchString(jobID) {
		return jobID
	}
	sum := sha256.Sum256([]byte(jobID))
	return "j-" + hex.EncodeToString(sum[:16])
}

// Create writes a new entry for an assignment (assignment.json then state.json, both
// fsynced). An existing entry of the same job is replaced (a new generation).
func (o *Outbox) Create(a workerproto.Assignment) (*Entry, error) {
	dir := filepath.Join(o.root, DirName(a.JobID))
	if err := os.RemoveAll(dir); err != nil {
		return nil, fmt.Errorf("failed to clear outbox entry: %w", err)
	}
	for _, sub := range []string{outDir, tmpDir} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create outbox entry: %w", err)
		}
	}
	e := &Entry{Dir: dir, Assignment: a, State: State{Stage: StageComputing, Generation: a.Lease.Generation}}
	if err := writeJSONAtomic(filepath.Join(dir, assignmentFile), a); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if err := e.Save(); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	syncDir(o.root)
	return e, nil
}

// Load reads every entry. Directories without a readable assignment.json (a crash during
// Create) are removed; other broken entries are returned as errors and left on disk.
func (o *Outbox) Load() ([]*Entry, []error) {
	items, err := os.ReadDir(o.root)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to read outbox: %w", err)}
	}
	var entries []*Entry
	var errs []error
	for _, it := range items {
		if !it.IsDir() {
			continue
		}
		dir := filepath.Join(o.root, it.Name())
		e := &Entry{Dir: dir}
		if err := readJSON(filepath.Join(dir, assignmentFile), &e.Assignment); err != nil {
			if errors.Is(err, os.ErrNotExist) || isSyntax(err) {
				_ = os.RemoveAll(dir)
				continue
			}
			errs = append(errs, fmt.Errorf("outbox entry %s: %w", it.Name(), err))
			continue
		}
		if err := readJSON(filepath.Join(dir, stateFile), &e.State); err != nil {
			// assignment written but not the state: the computation had not started.
			e.State = State{Stage: StageComputing, Generation: e.Assignment.Lease.Generation}
		}
		if e.State.Generation == 0 {
			e.State.Generation = e.Assignment.Lease.Generation
		}
		entries = append(entries, e)
	}
	return entries, errs
}

// Remove deletes an entry (the server acknowledged it, or it is void). The entry is void as
// soon as its assignment.json is gone (Load deletes such directories), so a file still held
// open by another process (Windows) only delays the cleanup: the rest of the directory is
// retried in the background for a while, then by the next Load.
func (o *Outbox) Remove(e *Entry) error {
	if err := os.Remove(filepath.Join(e.Dir, assignmentFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove outbox entry %s: %w", e.Dir, err)
	}
	if err := os.RemoveAll(e.Dir); err != nil {
		go retryRemoveAll(e.Dir)
	}
	return nil
}

// retryRemoveAll retries removing dir for about two minutes.
func retryRemoveAll(dir string) {
	for delay := time.Second; delay <= time.Minute; delay *= 2 {
		time.Sleep(delay)
		if os.RemoveAll(dir) == nil {
			return
		}
	}
}

// OutDir is the compute output directory of the entry.
func (e *Entry) OutDir() string { return filepath.Join(e.Dir, outDir) }

// WorkDir is a scratch directory of the entry (sprite frames...).
func (e *Entry) WorkDir() string { return filepath.Join(e.Dir, tmpDir) }

// ArtifactPath is the local file of an artifact.
func (e *Entry) ArtifactPath(name string) string { return filepath.Join(e.OutDir(), name) }

// Save writes state.json atomically.
func (e *Entry) Save() error {
	e.State.UpdatedAt = time.Now().UTC()
	return writeJSONAtomic(filepath.Join(e.Dir, stateFile), e.State)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func isSyntax(err error) bool {
	var se *json.SyntaxError
	var ue *json.UnmarshalTypeError
	return errors.As(err, &se) || errors.As(err, &ue) || err.Error() == "unexpected end of JSON input"
}

// writeJSONAtomic writes v to path through a temp file, fsync and rename.
func writeJSONAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode %s: %w", filepath.Base(path), err)
	}
	return WriteFileAtomic(path, b)
}

// WriteFileAtomic writes data to path through a temp file in the same directory, fsync
// and rename (a reader sees the old or the new content, never a partial one).
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("failed to sync %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to rename %s: %w", path, err)
	}
	ok = true
	syncDir(dir)
	return nil
}

// syncDir fsyncs a directory so a rename survives a power loss (no-op on Windows).
func syncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}
