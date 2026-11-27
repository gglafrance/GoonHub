package workers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"goonhub/pkg/workerproto"
)

// Staging stores the artifacts uploaded by remote workers until their job's complete:
//
//	<root>/remote/<job_id>/g<generation>/.<name>.part   upload in progress (appended)
//	<root>/remote/<job_id>/g<generation>/.<name>.meta   declared size and sha256 (JSON)
//	<root>/remote/<job_id>/g<generation>/<name>         uploaded, sha256 verified
//
// The root is on the generated-data file system, never under a media storage path. A
// generation's directory is only ever committed by a complete of that generation.
type Staging struct {
	root     string
	maxBytes int64
	locks    keyedMutex
}

// NewStaging creates the staging area under root (jobs.StagingRoot).
func NewStaging(root string, maxBytes int64) *Staging {
	return &Staging{root: root, maxBytes: maxBytes}
}

var jobIDRe = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// RemoteRoot is the directory of every remote job.
func (s *Staging) RemoteRoot() string { return filepath.Join(s.root, "remote") }

// Dir is the staging directory of one generation of a job.
func (s *Staging) Dir(jobID string, generation int64) string {
	return filepath.Join(s.RemoteRoot(), jobID, "g"+strconv.FormatInt(generation, 10))
}

type uploadMeta struct {
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	Complete bool   `json:"complete"`
}

func partPath(dir, name string) string { return filepath.Join(dir, "."+name+".part") }
func metaPath(dir, name string) string { return filepath.Join(dir, "."+name+".meta") }

func readMeta(dir, name string) (*uploadMeta, error) {
	raw, err := os.ReadFile(metaPath(dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m uploadMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil // corrupt sidecar: treated as unknown, the upload restarts from 0
	}
	return &m, nil
}

func writeMeta(dir, name string, m uploadMeta) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := metaPath(dir, name) + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, metaPath(dir, name))
}

func validateUpload(jobID, name string) error {
	if !jobIDRe.MatchString(jobID) {
		return errInvalid("invalid job id")
	}
	if err := workerproto.ValidateArtifactName(name); err != nil {
		return newError(http.StatusBadRequest, workerproto.CodeInvalidArtifactName, "%s", err.Error())
	}
	return nil
}

func diskError(err error) error {
	if errors.Is(err, syscall.ENOSPC) {
		return newError(http.StatusInsufficientStorage, workerproto.CodeServerDiskFull, "server disk full")
	}
	return fmt.Errorf("staging write failed: %w", err)
}

// Status returns what the server holds of an artifact (received 0 when unknown).
func (s *Staging) Status(jobID string, generation int64, name string) (workerproto.UploadStatus, error) {
	if err := validateUpload(jobID, name); err != nil {
		return workerproto.UploadStatus{}, err
	}
	unlock := s.locks.Lock(jobID + "/" + name)
	defer unlock()
	return s.statusLocked(s.Dir(jobID, generation), name)
}

func (s *Staging) statusLocked(dir, name string) (workerproto.UploadStatus, error) {
	st := workerproto.UploadStatus{Name: name}
	m, err := readMeta(dir, name)
	if err != nil {
		return st, fmt.Errorf("failed to read upload state: %w", err)
	}
	if m == nil {
		return st, nil
	}
	st.Size, st.SHA256 = m.Size, m.SHA256
	if m.Complete {
		if fi, err := os.Stat(filepath.Join(dir, name)); err == nil && fi.Size() == m.Size {
			st.Received, st.Complete = m.Size, true
			return st, nil
		}
		return workerproto.UploadStatus{Name: name}, nil // lost the file: start again
	}
	if fi, err := os.Stat(partPath(dir, name)); err == nil {
		st.Received = fi.Size()
	}
	return st, nil
}

// Put appends body at offset to an artifact upload (section 7.5) and returns the new status.
// When the whole file is there, it is hashed: a match makes it complete, a mismatch deletes
// it (422 sha256_mismatch).
func (s *Staging) Put(jobID string, generation int64, name string, offset, size int64, sum string, body io.Reader) (workerproto.UploadStatus, error) {
	if err := validateUpload(jobID, name); err != nil {
		return workerproto.UploadStatus{}, err
	}
	if size < 0 || offset < 0 {
		return workerproto.UploadStatus{}, errInvalid("invalid size or offset")
	}
	if size > s.maxBytes {
		return workerproto.UploadStatus{}, newError(http.StatusRequestEntityTooLarge, workerproto.CodeArtifactTooLarge,
			"artifact of %d bytes is larger than the %d bytes allowed", size, s.maxBytes)
	}
	sum = strings.ToLower(sum)
	if !workerproto.ValidSHA256(sum) {
		return workerproto.UploadStatus{}, errInvalid("invalid %s", workerproto.HeaderArtifactSHA256)
	}
	if offset > size {
		return workerproto.UploadStatus{}, errInvalid("offset beyond the artifact size")
	}

	unlock := s.locks.Lock(jobID + "/" + name)
	defer unlock()
	dir := s.Dir(jobID, generation)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return workerproto.UploadStatus{}, diskError(err)
	}
	current, err := s.statusLocked(dir, name)
	if err != nil {
		return current, err
	}
	sameDecl := current.Size == size && current.SHA256 == sum
	if current.Complete && sameDecl {
		return current, nil // already uploaded: idempotent
	}
	if offset == 0 {
		// A new declaration (or a restart): truncate.
		_ = os.Remove(filepath.Join(dir, name))
		if err := writeMeta(dir, name, uploadMeta{Size: size, SHA256: sum}); err != nil {
			return current, diskError(err)
		}
		f, err := os.OpenFile(partPath(dir, name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return current, diskError(err)
		}
		if err := f.Close(); err != nil {
			return current, diskError(err)
		}
		current = workerproto.UploadStatus{Name: name, Size: size, SHA256: sum}
	} else {
		if current.SHA256 == "" || !sameDecl || current.Complete {
			return current, &Error{Status: http.StatusConflict, Code: workerproto.CodeOffsetMismatch,
				Message: "the declared artifact differs from the upload in progress: start again at offset 0",
				Details: map[string]string{"received": strconv.FormatInt(current.Received, 10)}}
		}
		if offset != current.Received {
			return current, &Error{Status: http.StatusConflict, Code: workerproto.CodeOffsetMismatch,
				Message: fmt.Sprintf("Upload-Offset %d differs from the %d bytes received", offset, current.Received),
				Details: map[string]string{"received": strconv.FormatInt(current.Received, 10)}}
		}
	}

	f, err := os.OpenFile(partPath(dir, name), os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return current, diskError(err)
	}
	n, copyErr := io.Copy(f, io.LimitReader(body, size-offset))
	syncErr := f.Sync() // keep every byte received, even on a broken connection
	closeErr := f.Close()
	current.Received = offset + n
	if copyErr != nil {
		if errors.Is(copyErr, syscall.ENOSPC) {
			return current, diskError(copyErr)
		}
		return current, fmt.Errorf("upload interrupted after %d bytes: %w", current.Received, copyErr)
	}
	if syncErr != nil {
		return current, diskError(syncErr)
	}
	if closeErr != nil {
		return current, diskError(closeErr)
	}
	if current.Received < size {
		return current, nil
	}

	got, gotSize, err := workerproto.SHA256File(partPath(dir, name))
	if err != nil {
		return current, fmt.Errorf("failed to hash artifact: %w", err)
	}
	if got != sum || gotSize != size {
		_ = os.Remove(partPath(dir, name))
		_ = os.Remove(metaPath(dir, name))
		return workerproto.UploadStatus{Name: name, Size: size, SHA256: sum}, newError(http.StatusUnprocessableEntity,
			workerproto.CodeSHA256Mismatch, "artifact %s does not match its sha256", name)
	}
	if err := os.Rename(partPath(dir, name), filepath.Join(dir, name)); err != nil {
		return current, diskError(err)
	}
	if err := writeMeta(dir, name, uploadMeta{Size: size, SHA256: sum, Complete: true}); err != nil {
		return current, diskError(err)
	}
	current.Complete = true
	return current, nil
}

// Verify checks that every artifact is complete with its declared size and sha256. It
// returns, per problem artifact, "missing", "partial" or "sha256" (empty map = all good).
func (s *Staging) Verify(jobID string, generation int64, refs []workerproto.ArtifactRef) (map[string]string, error) {
	problems := map[string]string{}
	dir := s.Dir(jobID, generation)
	for _, ref := range refs {
		unlock := s.locks.Lock(jobID + "/" + ref.Name)
		st, err := s.statusLocked(dir, ref.Name)
		unlock()
		if err != nil {
			return nil, err
		}
		switch {
		case st.Complete && st.Size == ref.Size && st.SHA256 == strings.ToLower(ref.SHA256):
		case st.Complete:
			problems[ref.Name] = "sha256"
		case st.Received > 0 || st.SHA256 != "":
			problems[ref.Name] = "partial"
		default:
			problems[ref.Name] = "missing"
		}
	}
	return problems, nil
}

// RemoveJob deletes every staged generation of a job.
func (s *Staging) RemoveJob(jobID string) error {
	if !jobIDRe.MatchString(jobID) {
		return nil
	}
	return os.RemoveAll(filepath.Join(s.RemoteRoot(), jobID))
}

// StagedGeneration is one staged generation directory.
type StagedGeneration struct {
	JobID      string
	Generation int64
	Dir        string
	ModTime    time.Time
}

// List returns every staged generation directory.
func (s *Staging) List() ([]StagedGeneration, error) {
	jobsDirs, err := os.ReadDir(s.RemoteRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []StagedGeneration
	for _, jd := range jobsDirs {
		if !jd.IsDir() || !jobIDRe.MatchString(jd.Name()) {
			continue
		}
		jobDir := filepath.Join(s.RemoteRoot(), jd.Name())
		gens, err := os.ReadDir(jobDir)
		if err != nil {
			continue
		}
		if len(gens) == 0 {
			_ = os.Remove(jobDir)
			continue
		}
		for _, g := range gens {
			gen, err := strconv.ParseInt(strings.TrimPrefix(g.Name(), "g"), 10, 64)
			if !g.IsDir() || !strings.HasPrefix(g.Name(), "g") || err != nil {
				continue
			}
			dir := filepath.Join(jobDir, g.Name())
			mod := time.Time{}
			if fi, err := os.Stat(dir); err == nil {
				mod = fi.ModTime()
			}
			// The newest file counts (an upload in progress keeps the directory alive).
			if entries, err := os.ReadDir(dir); err == nil {
				for _, e := range entries {
					if fi, err := e.Info(); err == nil && fi.ModTime().After(mod) {
						mod = fi.ModTime()
					}
				}
			}
			out = append(out, StagedGeneration{JobID: jd.Name(), Generation: gen, Dir: dir, ModTime: mod})
		}
	}
	return out, nil
}

// RemoveGeneration deletes one staged generation.
func (s *Staging) RemoveGeneration(g StagedGeneration) error {
	if err := os.RemoveAll(g.Dir); err != nil {
		return err
	}
	_ = os.Remove(filepath.Dir(g.Dir)) // the job directory, when now empty
	return nil
}
