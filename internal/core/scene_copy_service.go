package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/pkg/oshash"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MaxBulkSceneCopies caps the copies of one bulk action (and of "select all").
const MaxBulkSceneCopies = 500

// SceneCopyService triages the copies recorded by the library scan (Duplicates page > Copies):
// list them, make a copy the scene's main file, turn it into its own scene, forget it, or
// delete its file. Deleting a copy's file is the only destructive action: it is refused on a
// read-only storage path, for the scene's main file, and when the copy and the main file no
// longer have the same content (both re-hashed right before), so the last good file of a video
// is never deleted.
type SceneCopyService struct {
	copyRepo           data.SceneCopyRepository
	sceneRepo          data.SceneRepository
	storagePathService *StoragePathService
	guard              *MediaFileGuard
	processingService  *SceneProcessingService
	indexer            SceneIndexer
	logger             *zap.Logger

	// hashFile and statFile read the files (replaced in tests)
	hashFile func(path string) (string, error)
	statFile func(path string) (fs.FileInfo, error)

	// mu serialises the actions on copies (a swap and a deletion never run together)
	mu sync.Mutex
}

// NewSceneCopyService creates the copy triage service.
func NewSceneCopyService(
	copyRepo data.SceneCopyRepository,
	sceneRepo data.SceneRepository,
	storagePathService *StoragePathService,
	guard *MediaFileGuard,
	processingService *SceneProcessingService,
	logger *zap.Logger,
) *SceneCopyService {
	return &SceneCopyService{
		copyRepo:           copyRepo,
		sceneRepo:          sceneRepo,
		storagePathService: storagePathService,
		guard:              guard,
		processingService:  processingService,
		logger:             logger.With(zap.String("component", "scene_copy_service")),
		hashFile:           oshash.FromFile,
		statFile:           os.Stat,
	}
}

// SetIndexer sets the search indexer used to re-index a scene whose main file changed.
func (s *SceneCopyService) SetIndexer(indexer SceneIndexer) {
	s.indexer = indexer
}

// SceneCopyView is a copy in the triage list.
type SceneCopyView struct {
	data.SceneCopy
	// NotSeenInLastScan is true when the last completed scan did not see the file
	NotSeenInLastScan bool `json:"not_seen_in_last_scan"`
}

// SceneWithCopies is a scene and its copies matching the filter.
type SceneWithCopies struct {
	data.SceneWithCopiesRow
	Copies []SceneCopyView `json:"copies"`
}

// SceneCopyList is a page of the triage list.
type SceneCopyList struct {
	Scenes     []SceneWithCopies    `json:"scenes"`
	Totals     data.SceneCopyTotals `json:"totals"`
	LastScanAt *time.Time           `json:"last_scan_at"`
}

// List returns a page of the scenes with copies matching the filter, with their copies and
// the totals of the filter.
func (s *SceneCopyService) List(q data.SceneCopyQuery) (*SceneCopyList, error) {
	totals, err := s.copyRepo.CopyTotals(q.SceneCopyFilter)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count scene copies", err)
	}
	rows, err := s.copyRepo.ListScenesWithCopies(q)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list scenes with copies", err)
	}
	lastScan, err := s.copyRepo.LastCompletedScanStart()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to read the last scan", err)
	}
	ids := make([]uint, len(rows))
	for i, r := range rows {
		ids[i] = r.SceneID
	}
	copies, err := s.copyRepo.ListCopiesOfScenes(ids, q.SceneCopyFilter)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list scene copies", err)
	}
	byScene := make(map[uint][]SceneCopyView, len(rows))
	for _, c := range copies {
		v := SceneCopyView{SceneCopy: c, NotSeenInLastScan: lastScan != nil && c.LastSeenAt.Before(*lastScan)}
		byScene[c.SceneID] = append(byScene[c.SceneID], v)
	}
	out := &SceneCopyList{Scenes: make([]SceneWithCopies, len(rows)), Totals: totals, LastScanAt: lastScan}
	for i, r := range rows {
		cs := byScene[r.SceneID]
		if cs == nil {
			cs = []SceneCopyView{}
		}
		out.Scenes[i] = SceneWithCopies{SceneWithCopiesRow: r, Copies: cs}
	}
	return out, nil
}

// SelectableIDs returns the IDs of the copies matching the filter (at most limit) and their
// total, for "select all".
func (s *SceneCopyService) SelectableIDs(f data.SceneCopyFilter, limit int) ([]uint, int64, error) {
	ids, total, err := s.copyRepo.ListCopyIDs(f, limit)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("failed to list scene copies", err)
	}
	return ids, total, nil
}

func (s *SceneCopyService) getCopy(id uint) (*data.SceneCopy, *data.SceneFileRow, error) {
	c, err := s.copyRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, apperrors.NewNotFoundError("scene copy", id)
		}
		return nil, nil, apperrors.NewInternalError("failed to get the scene copy", err)
	}
	scene, err := s.copyRepo.GetSceneFile(c.SceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, apperrors.NewNotFoundError("scene", c.SceneID)
		}
		return nil, nil, apperrors.NewInternalError("failed to get the scene", err)
	}
	return c, scene, nil
}

// regularFile returns the info of the file at path when it is a regular file on disk.
func (s *SceneCopyService) regularFile(path string) (fs.FileInfo, error) {
	info, err := s.statFile(path)
	if err != nil {
		return nil, apperrors.NewValidationError(fmt.Sprintf("the file %s cannot be read: %v", path, err))
	}
	if !info.Mode().IsRegular() {
		return nil, apperrors.NewValidationError(fmt.Sprintf("%s is not a regular file", path))
	}
	return info, nil
}

// storagePathOf returns the storage path a file belongs to: the recorded one, else the
// configured path containing the file (nil when none does).
func (s *SceneCopyService) storagePathOf(recorded *uint, path string) *uint {
	if recorded != nil || s.storagePathService == nil {
		return recorded
	}
	paths, err := s.storagePathService.List()
	if err != nil {
		return nil
	}
	for i := range paths {
		if pathWithin(path, paths[i].Path) {
			id := paths[i].ID
			return &id
		}
	}
	return nil
}

// reindex refreshes the search index entry of a scene that is not in the trash.
func (s *SceneCopyService) reindex(sceneID uint) {
	if s.indexer == nil {
		return
	}
	scene, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		return // trashed or gone: not in the index
	}
	if err := s.indexer.IndexScene(scene); err != nil {
		s.logger.Warn("Failed to re-index scene", zap.Uint("scene_id", sceneID), zap.Error(err))
	}
}

// MakeMain makes a copy the scene's main file; the old main file becomes a copy. The scene ID
// does not change (markers, tags, history are kept). No file is touched.
func (s *SceneCopyService) MakeMain(copyID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, scene, err := s.getCopy(copyID)
	if err != nil {
		return err
	}
	if _, err := s.regularFile(c.StoredPath); err != nil {
		return err
	}
	if err := s.copyRepo.SwapWithMain(c.ID, s.storagePathOf(c.StoragePathID, c.StoredPath)); err != nil {
		return apperrors.NewInternalError("failed to make the copy the main file", err)
	}
	s.logger.Info("Scene copy made the main file",
		zap.Uint("scene_id", scene.ID),
		zap.String("new_main", c.StoredPath),
		zap.String("old_main", scene.StoredPath),
	)
	s.reindex(scene.ID)
	return nil
}

// Separate turns a copy into its own scene, created as the scan creates scenes and submitted
// for processing; the copy row is removed. No file is touched.
func (s *SceneCopyService) Separate(copyID uint) (*data.Scene, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, _, err := s.getCopy(copyID)
	if err != nil {
		return nil, err
	}
	info, err := s.regularFile(c.StoredPath)
	if err != nil {
		return nil, err
	}
	scene := newScannedSceneRecord(c.StoredPath, info, s.storagePathOf(c.StoragePathID, c.StoredPath))
	if err := s.copyRepo.PromoteToScene(c.ID, scene); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("scene copy", copyID)
		}
		return nil, apperrors.NewInternalError("failed to create a scene for the copy", err)
	}
	s.logger.Info("Scene copy separated into its own scene",
		zap.Uint("copy_of_scene_id", c.SceneID),
		zap.Uint("scene_id", scene.ID),
		zap.String("path", scene.StoredPath),
	)
	if s.indexer != nil {
		if err := s.indexer.IndexScene(scene); err != nil {
			s.logger.Warn("Failed to index the new scene", zap.Uint("scene_id", scene.ID), zap.Error(err))
		}
	}
	if s.processingService != nil {
		if err := s.processingService.SubmitScene(scene.ID, scene.StoredPath); err != nil {
			s.logger.Warn("Failed to submit the new scene for processing", zap.Uint("scene_id", scene.ID), zap.Error(err))
		}
	}
	return scene, nil
}

// Forget removes a copy row only: the file stays on disk (a later scan records it again).
func (s *SceneCopyService) Forget(copyID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.copyRepo.Delete(copyID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.NewNotFoundError("scene copy", copyID)
		}
		return apperrors.NewInternalError("failed to forget the scene copy", err)
	}
	return nil
}

// SceneCopyBulkFailure is a copy a bulk action could not handle.
type SceneCopyBulkFailure struct {
	CopyID uint   `json:"copy_id"`
	Error  string `json:"error"`
}

// SceneCopyBulkResult is the per-copy outcome of a bulk action.
type SceneCopyBulkResult struct {
	Done   []uint                 `json:"done"`
	Failed []SceneCopyBulkFailure `json:"failed"`
	// FreedBytes is the size of the deleted files (bulk delete)
	FreedBytes int64 `json:"freed_bytes"`
}

func newSceneCopyBulkResult() *SceneCopyBulkResult {
	return &SceneCopyBulkResult{Done: []uint{}, Failed: []SceneCopyBulkFailure{}}
}

// ForgetBulk removes the copy rows of the given IDs (never the files).
func (s *SceneCopyService) ForgetBulk(ids []uint) *SceneCopyBulkResult {
	res := newSceneCopyBulkResult()
	for _, id := range uniqueIDs(ids) {
		if err := s.Forget(id); err != nil {
			res.Failed = append(res.Failed, SceneCopyBulkFailure{CopyID: id, Error: err.Error()})
			continue
		}
		res.Done = append(res.Done, id)
	}
	return res
}

// DeleteFile permanently deletes a copy's file from disk, then its row. Refused when the copy
// is the scene's main file (same path or same file on disk), when its storage path is
// read-only, and when the copy and the scene's main file no longer have the same oshash (both
// re-hashed now; an unreadable main file refuses too). Returns the size of the deleted file.
func (s *SceneCopyService) DeleteFile(copyID uint) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, scene, err := s.getCopy(copyID)
	if err != nil {
		return 0, err
	}
	if scene.StoredPath == "" {
		return 0, apperrors.NewValidationError("the scene has no main file: the copy is kept")
	}
	if filepath.Clean(c.StoredPath) == filepath.Clean(scene.StoredPath) {
		return 0, apperrors.NewValidationError("this path is the scene's main file: it is never deleted here")
	}
	if err := s.guard.CheckModifiable(c.StoragePathID, c.StoredPath); err != nil {
		return 0, err
	}
	copyInfo, err := s.regularFile(c.StoredPath)
	if err != nil {
		return 0, err
	}
	mainInfo, err := s.statFile(scene.StoredPath)
	if err != nil {
		return 0, apperrors.NewValidationError(fmt.Sprintf("the scene's main file cannot be read (%v): the copy is kept", err))
	}
	if os.SameFile(copyInfo, mainInfo) {
		return 0, apperrors.NewValidationError("the copy and the main file are the same file on disk: it is never deleted here")
	}
	copyHash, err := s.hashFile(c.StoredPath)
	if err != nil {
		return 0, apperrors.NewValidationError(fmt.Sprintf("the copy cannot be read (%v): nothing deleted", err))
	}
	mainHash, err := s.hashFile(scene.StoredPath)
	if err != nil {
		return 0, apperrors.NewValidationError(fmt.Sprintf("the scene's main file cannot be read (%v): the copy is kept", err))
	}
	if copyHash != mainHash {
		return 0, apperrors.NewValidationError("the copy no longer has the same content as the scene's main file: it is kept")
	}

	if err := s.guard.RemoveMediaFile(c.StoragePathID, c.StoredPath); err != nil {
		return 0, err
	}
	if err := s.copyRepo.Delete(c.ID); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Warn("Copy file deleted but its row could not be removed", zap.Uint("copy_id", c.ID), zap.Error(err))
	}
	s.logger.Info("Scene copy file deleted",
		zap.Uint("copy_id", c.ID),
		zap.Uint("scene_id", scene.ID),
		zap.String("path", c.StoredPath),
		zap.Int64("size", copyInfo.Size()),
		zap.String("kept_main", scene.StoredPath),
	)
	return copyInfo.Size(), nil
}

// DeleteFilesBulk deletes the files of the given copies, each with the checks of DeleteFile.
func (s *SceneCopyService) DeleteFilesBulk(ids []uint) *SceneCopyBulkResult {
	res := newSceneCopyBulkResult()
	for _, id := range uniqueIDs(ids) {
		size, err := s.DeleteFile(id)
		if err != nil {
			res.Failed = append(res.Failed, SceneCopyBulkFailure{CopyID: id, Error: err.Error()})
			continue
		}
		res.Done = append(res.Done, id)
		res.FreedBytes += size
	}
	return res
}
