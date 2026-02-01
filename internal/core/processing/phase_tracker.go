package processing

import (
	"fmt"
	"goonhub/internal/data"
	"sync"
)

// PhaseTracker manages trigger configuration cache and phase state tracking
type PhaseTracker struct {
	triggerConfigRepo data.TriggerConfigRepository
	triggerCache      []data.TriggerConfigRecord
	triggerCacheMu    sync.RWMutex
	phases            sync.Map // map[sceneID uint]*PhaseState
}

// NewPhaseTracker creates a new PhaseTracker
func NewPhaseTracker(triggerConfigRepo data.TriggerConfigRepository) *PhaseTracker {
	return &PhaseTracker{
		triggerConfigRepo: triggerConfigRepo,
	}
}

// RefreshTriggerCache reloads the trigger configuration from the database
func (pt *PhaseTracker) RefreshTriggerCache() error {
	if pt.triggerConfigRepo == nil {
		return nil
	}
	configs, err := pt.triggerConfigRepo.GetAll()
	if err != nil {
		return fmt.Errorf("failed to load trigger configs: %w", err)
	}
	pt.triggerCacheMu.Lock()
	pt.triggerCache = configs
	pt.triggerCacheMu.Unlock()
	return nil
}

// GetTriggerForPhase returns the trigger configuration for a specific phase
func (pt *PhaseTracker) GetTriggerForPhase(phase string) *data.TriggerConfigRecord {
	pt.triggerCacheMu.RLock()
	defer pt.triggerCacheMu.RUnlock()
	for i := range pt.triggerCache {
		if pt.triggerCache[i].Phase == phase {
			return &pt.triggerCache[i]
		}
	}
	return nil
}

// ShouldAutoDispatch returns whether a phase should be automatically dispatched
func (pt *PhaseTracker) ShouldAutoDispatch(phase string) bool {
	trigger := pt.GetTriggerForPhase(phase)
	if trigger == nil {
		// Default behavior: metadata=on_import, thumbnail/sprites=after_job(metadata)
		return true
	}
	return trigger.TriggerType == "on_import" || trigger.TriggerType == "after_job"
}

// GetPhasesTriggeredAfter returns phases that should be triggered after a completed phase
func (pt *PhaseTracker) GetPhasesTriggeredAfter(completedPhase string) []string {
	pt.triggerCacheMu.RLock()
	defer pt.triggerCacheMu.RUnlock()

	var phases []string
	for _, cfg := range pt.triggerCache {
		if cfg.TriggerType == "after_job" && cfg.AfterPhase != nil && *cfg.AfterPhase == completedPhase {
			phases = append(phases, cfg.Phase)
		}
	}
	return phases
}

// InitPhaseState initializes phase state tracking for a scene
func (pt *PhaseTracker) InitPhaseState(sceneID uint) {
	pt.phases.Store(sceneID, &PhaseState{})
}

// GetPhaseState retrieves the phase state for a scene
func (pt *PhaseTracker) GetPhaseState(sceneID uint) (*PhaseState, bool) {
	val, ok := pt.phases.Load(sceneID)
	if !ok {
		return nil, false
	}
	return val.(*PhaseState), true
}

// MarkPhaseComplete marks a phase as complete for a scene
func (pt *PhaseTracker) MarkPhaseComplete(sceneID uint, phase string) {
	val, ok := pt.phases.Load(sceneID)
	if !ok {
		return
	}
	state := val.(*PhaseState)
	switch phase {
	case "thumbnail":
		state.ThumbnailDone = true
	case "sprites":
		state.SpritesDone = true
	}
}

// ClearPhaseState removes phase state tracking for a scene
func (pt *PhaseTracker) ClearPhaseState(sceneID uint) {
	pt.phases.Delete(sceneID)
}

// CheckAllPhasesComplete checks if all phases in the pipeline are complete for a scene
// Returns true if all phases are complete and the scene should be marked as completed
func (pt *PhaseTracker) CheckAllPhasesComplete(sceneID uint, completedPhase string) bool {
	state, ok := pt.GetPhaseState(sceneID)
	if !ok {
		// No phase state means this was a standalone trigger (manual/scheduled)
		// or metadata completed with no auto-follow phases
		if completedPhase == "metadata" {
			phasesAfter := pt.GetPhasesTriggeredAfter("metadata")
			return len(phasesAfter) == 0
		}
		return false
	}

	// Determine which phases are part of the auto-pipeline
	phasesAfterMeta := pt.GetPhasesTriggeredAfter("metadata")
	thumbnailInPipeline := false
	spritesInPipeline := false
	for _, p := range phasesAfterMeta {
		if p == "thumbnail" {
			thumbnailInPipeline = true
		}
		if p == "sprites" {
			spritesInPipeline = true
		}
	}

	// Check completion: only phases in the pipeline matter
	thumbnailReady := !thumbnailInPipeline || state.ThumbnailDone
	spritesReady := !spritesInPipeline || state.SpritesDone

	if thumbnailReady && spritesReady {
		pt.ClearPhaseState(sceneID)
		return true
	}

	return false
}
