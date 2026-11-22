package jobs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"goonhub/pkg/workerproto"

	"go.uber.org/zap"
)

// The commit step of the remotable phases. A phase is computed by pkg/compute into a staging
// directory (locally by the server's jobs, or remotely by a worker whose uploaded artifacts
// the server verified), then committed here: files moved into their final directory, then
// the database (and ClickHouse for fingerprints) updated.
//
// Commits are idempotent: committing the same result again is harmless. A file already moved
// by an earlier, interrupted commit (missing from the artifact directory but present at its
// destination) counts as moved.

// StagingRoot returns the directory under which job outputs wait for their commit: the
// configured processing.staging_dir, else "staging" next to the sprite directory (same file
// system as the generated folders, so the commit is a rename).
func StagingRoot(configured, spriteDir string) string {
	if configured != "" {
		return configured
	}
	return filepath.Join(filepath.Dir(filepath.Clean(spriteDir)), "staging")
}

// LocalStagingDir is the staging directory of a job computed by the server itself.
func LocalStagingDir(stagingRoot, jobID string) string {
	return filepath.Join(stagingRoot, "local", jobID)
}

// MoveArtifact moves artifactDir/name to destDir/name, replacing the destination atomically
// (rename; across file systems: copy to a temporary file in destDir, fsync, rename).
func MoveArtifact(artifactDir, destDir, name string) error {
	if err := workerproto.ValidateArtifactName(name); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}
	src := filepath.Join(artifactDir, name)
	dst := filepath.Join(destDir, name)
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			if _, dstErr := os.Stat(dst); dstErr == nil {
				return nil // moved by an earlier commit
			}
			return fmt.Errorf("artifact %s is missing", name)
		}
		return fmt.Errorf("failed to stat artifact %s: %w", name, err)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyThenRename(src, dst); err != nil {
		return fmt.Errorf("failed to move artifact %s: %w", name, err)
	}
	if err := os.Remove(src); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove staged artifact %s: %w", name, err)
	}
	return nil
}

func copyThenRename(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp-"+hex.EncodeToString(suffix))
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------------------
// Sprites
// ---------------------------------------------------------------------------------------

// SpritesCommitRepo is the scene repository part the sprites commit needs.
type SpritesCommitRepo interface {
	UpdateSprites(id uint, spriteSheetPath, vttPath string, spriteSheetCount int) error
}

// CommitSprites moves the sprite sheets into spriteDir, then the VTT into vttDir (a VTT
// never points to sheets not yet in place), then records them on the scene.
func CommitSprites(repo SpritesCommitRepo, spriteDir, vttDir string, sceneID uint, res *workerproto.SpritesResult, artifactDir string) (*SpritesResult, error) {
	if res == nil {
		return nil, fmt.Errorf("sprites commit: no result")
	}
	if err := res.Validate(sceneID); err != nil {
		return nil, err
	}
	for _, sheet := range res.SpriteSheets {
		if err := MoveArtifact(artifactDir, spriteDir, sheet); err != nil {
			return nil, fmt.Errorf("failed to commit sprite sheet: %w", err)
		}
	}
	if err := MoveArtifact(artifactDir, vttDir, res.VTT); err != nil {
		return nil, fmt.Errorf("failed to commit VTT file: %w", err)
	}

	spriteSheetPath := ""
	if len(res.SpriteSheets) > 0 {
		spriteSheetPath = filepath.Join(spriteDir, res.SpriteSheets[0])
	}
	vttPath := filepath.Join(vttDir, res.VTT)
	if err := repo.UpdateSprites(sceneID, spriteSheetPath, vttPath, len(res.SpriteSheets)); err != nil {
		return nil, fmt.Errorf("failed to update sprites: %w", err)
	}
	return &SpritesResult{
		SpriteSheetPath:  spriteSheetPath,
		VttPath:          vttPath,
		SpriteSheetCount: len(res.SpriteSheets),
	}, nil
}

// ---------------------------------------------------------------------------------------
// Animated thumbnails
// ---------------------------------------------------------------------------------------

// MarkerClipCommitRepo is the marker repository part the animated thumbnails commit needs.
type MarkerClipCommitRepo interface {
	UpdateAnimatedThumbnailPath(id uint, path string) error
}

// PreviewCommitRepo is the scene repository part the animated thumbnails commit needs.
type PreviewCommitRepo interface {
	UpdatePreviewVideoPath(id uint, previewVideoPath string) error
}

// AnimatedThumbnailsCommit holds what CommitAnimatedThumbnails writes to.
type AnimatedThumbnailsCommit struct {
	Markers    MarkerClipCommitRepo
	Scenes     PreviewCommitRepo
	MarkerDir  string // processing.marker_thumbnail_dir
	PreviewDir string // processing.scene_preview_dir
}

// CommitAnimatedThumbnails moves each marker clip into MarkerDir and records it on its
// marker, then the preview into PreviewDir and records it on the scene. A failure on one
// item does not stop the others (best effort, as the generation always was): the returned
// error joins every failure, and committed counts the marker clips recorded. The caller
// validates the result against its parameters first (AnimatedThumbnailsResult.Validate).
func CommitAnimatedThumbnails(c AnimatedThumbnailsCommit, sceneID uint, res *workerproto.AnimatedThumbnailsResult, artifactDir string) (committed int, previewCommitted bool, err error) {
	if res == nil {
		return 0, false, nil
	}
	var errs []error
	for _, m := range res.Markers {
		if moveErr := MoveArtifact(artifactDir, c.MarkerDir, m.File); moveErr != nil {
			errs = append(errs, fmt.Errorf("marker %d: %w", m.MarkerID, moveErr))
			continue
		}
		if dbErr := c.Markers.UpdateAnimatedThumbnailPath(m.MarkerID, m.File); dbErr != nil {
			errs = append(errs, fmt.Errorf("marker %d: failed to update marker with animated thumbnail path: %w", m.MarkerID, dbErr))
			continue
		}
		committed++
	}
	if res.Preview != nil {
		if moveErr := MoveArtifact(artifactDir, c.PreviewDir, res.Preview.File); moveErr != nil {
			errs = append(errs, fmt.Errorf("scene preview: %w", moveErr))
		} else if dbErr := c.Scenes.UpdatePreviewVideoPath(sceneID, res.Preview.File); dbErr != nil {
			errs = append(errs, fmt.Errorf("failed to update scene with preview path: %w", dbErr))
		} else {
			previewCommitted = true
		}
	}
	return committed, previewCommitted, errors.Join(errs...)
}

// ---------------------------------------------------------------------------------------
// Fingerprint
// ---------------------------------------------------------------------------------------

// FingerprintCommitRepo is the scene repository part the fingerprint commit needs.
type FingerprintCommitRepo interface {
	UpdateFingerprint(id uint, fpType string, audioFP []byte, visualFP []byte) error
}

// MatchResult is a fingerprint match between two scenes.
type MatchResult struct {
	SceneID         uint
	ConfidenceScore float64
	MatchType       string // "audio" or "visual"
}

// FingerprintMatcher indexes fingerprints (ClickHouse) and finds/processes duplicates.
type FingerprintMatcher interface {
	IndexFingerprint(sceneID uint, fpType string, audioFP []int32, visualFP []uint64) error
	FindMatches(sceneID uint, fpType string, audioFP []int32, visualFP []uint64) ([]MatchResult, error)
	ProcessMatches(sceneID uint, matches []MatchResult) error
}

// FingerprintCommitter commits fingerprint results: PostgreSQL, then matching and indexing
// when a matcher is set. Commits are serialized (one at a time), so two scenes matching each
// other never create duplicate groups concurrently, wherever the commits come from (local
// fingerprint jobs, remote completes).
type FingerprintCommitter struct {
	repo    FingerprintCommitRepo
	logger  *zap.Logger
	mu      sync.Mutex
	matchMu sync.RWMutex
	matcher FingerprintMatcher
}

// NewFingerprintCommitter creates a FingerprintCommitter.
func NewFingerprintCommitter(repo FingerprintCommitRepo, logger *zap.Logger) *FingerprintCommitter {
	return &FingerprintCommitter{repo: repo, logger: logger}
}

// SetMatcher sets the matcher (the matching service is wired after the pools start).
func (c *FingerprintCommitter) SetMatcher(m FingerprintMatcher) {
	c.matchMu.Lock()
	defer c.matchMu.Unlock()
	c.matcher = m
}

func (c *FingerprintCommitter) getMatcher() FingerprintMatcher {
	c.matchMu.RLock()
	defer c.matchMu.RUnlock()
	return c.matcher
}

// Commit saves the fingerprint on the scene, then (with a matcher) finds the matches of
// each fingerprint type, indexes it and processes the deduplicated matches. Matching and
// indexing failures are logged (best effort, as before); the returned error is the
// PostgreSQL save error, if any (matching still runs on the extracted data).
func (c *FingerprintCommitter) Commit(sceneID uint, res *workerproto.FingerprintResult) error {
	if res == nil {
		return fmt.Errorf("fingerprint commit: no result")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	var audioBytes, visualBytes []byte
	if len(res.AudioFingerprint) > 0 {
		audioBytes = res.AudioFingerprint.Bytes()
	}
	if len(res.VisualFingerprint) > 0 {
		visualBytes = res.VisualFingerprint.Bytes()
	}
	var saveErr error
	if err := c.repo.UpdateFingerprint(sceneID, res.FingerprintTypeLabel(), audioBytes, visualBytes); err != nil {
		saveErr = fmt.Errorf("failed to save fingerprint to database: %w", err)
	}

	matcher := c.getMatcher()
	if matcher == nil {
		return saveErr
	}

	var allMatches []MatchResult
	if len(res.AudioFingerprint) > 0 {
		audio := []int32(res.AudioFingerprint)
		matches, err := matcher.FindMatches(sceneID, "audio", audio, nil)
		if err != nil {
			c.logger.Error("Failed to find audio fingerprint matches", zap.Uint("scene_id", sceneID), zap.Error(err))
		} else {
			allMatches = append(allMatches, matches...)
		}
		if err := matcher.IndexFingerprint(sceneID, "audio", audio, nil); err != nil {
			c.logger.Error("Failed to index audio fingerprint", zap.Uint("scene_id", sceneID), zap.Error(err))
		}
	}
	if len(res.VisualFingerprint) > 0 {
		visual := []uint64(res.VisualFingerprint)
		matches, err := matcher.FindMatches(sceneID, "visual", nil, visual)
		if err != nil {
			c.logger.Error("Failed to find visual fingerprint matches", zap.Uint("scene_id", sceneID), zap.Error(err))
		} else {
			allMatches = append(allMatches, matches...)
		}
		if err := matcher.IndexFingerprint(sceneID, "visual", nil, visual); err != nil {
			c.logger.Error("Failed to index visual fingerprint", zap.Uint("scene_id", sceneID), zap.Error(err))
		}
	}

	allMatches = DeduplicateMatches(allMatches)
	if len(allMatches) > 0 {
		if err := matcher.ProcessMatches(sceneID, allMatches); err != nil {
			c.logger.Error("Failed to process fingerprint matches",
				zap.Uint("scene_id", sceneID),
				zap.Int("match_count", len(allMatches)),
				zap.Error(err),
			)
		} else {
			c.logger.Info("Fingerprint matches processed",
				zap.Uint("scene_id", sceneID),
				zap.Int("match_count", len(allMatches)),
			)
		}
	}
	return saveErr
}

// DeduplicateMatches keeps the highest confidence match per scene ID.
func DeduplicateMatches(matches []MatchResult) []MatchResult {
	if len(matches) <= 1 {
		return matches
	}
	best := make(map[uint]MatchResult, len(matches))
	for _, m := range matches {
		if existing, ok := best[m.SceneID]; !ok || m.ConfidenceScore > existing.ConfidenceScore {
			best[m.SceneID] = m
		}
	}
	result := make([]MatchResult, 0, len(best))
	for _, m := range best {
		result = append(result, m)
	}
	return result
}
