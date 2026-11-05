package validators

import "fmt"

// Valid phase constants
var (
	// AllPhases includes all processing phases including scan
	AllPhases = map[string]bool{"metadata": true, "thumbnail": true, "sprites": true, "animated_thumbnails": true, "fingerprint": true, "scan": true}

	// ProcessingPhases includes only scene processing phases (not scan)
	ProcessingPhases = map[string]bool{"metadata": true, "thumbnail": true, "sprites": true, "animated_thumbnails": true, "fingerprint": true}

	// TriggerTypes includes all valid trigger types
	TriggerTypes = map[string]bool{"on_import": true, "after_job": true, "manual": true, "scheduled": true}

	// ScanTriggerTypes includes trigger types valid for scan phase
	ScanTriggerTypes = map[string]bool{"manual": true, "scheduled": true}

	// JobModes includes valid bulk job modes
	JobModes = map[string]bool{"missing": true, "all": true}

	// ForceTargets includes valid force target values for animated_thumbnails phase
	ForceTargets = map[string]bool{"markers": true, "previews": true, "both": true}
)

// ValidatePhase validates a phase is one of the allowed phases
func ValidatePhase(phase string) error {
	if !AllPhases[phase] {
		return fmt.Errorf("phase must be one of: metadata, thumbnail, sprites, animated_thumbnails, fingerprint, scan")
	}
	return nil
}

// ValidateProcessingPhase validates a phase is one of the scene processing phases
func ValidateProcessingPhase(phase string) error {
	if !ProcessingPhases[phase] {
		return fmt.Errorf("phase must be one of: metadata, thumbnail, sprites, animated_thumbnails, fingerprint")
	}
	return nil
}

// ValidateTriggerType validates a trigger type for a given phase
func ValidateTriggerType(phase, triggerType string) error {
	if phase == "scan" {
		if !ScanTriggerTypes[triggerType] {
			return fmt.Errorf("scan phase only supports manual or scheduled triggers")
		}
	} else {
		if !TriggerTypes[triggerType] {
			return fmt.Errorf("trigger_type must be one of: on_import, after_job, manual, scheduled")
		}
	}
	return nil
}

// ValidateOnImportTrigger validates that on_import is only used with metadata
func ValidateOnImportTrigger(phase, triggerType string) error {
	if triggerType == "on_import" && phase != "metadata" {
		return fmt.Errorf("only metadata phase can use on_import trigger")
	}
	return nil
}

// ValidateAfterJobTrigger validates after_job trigger configuration
func ValidateAfterJobTrigger(phase string, afterPhase *string) error {
	if afterPhase == nil || *afterPhase == "" {
		return fmt.Errorf("after_phase is required when trigger_type is after_job")
	}
	if !ProcessingPhases[*afterPhase] {
		return fmt.Errorf("after_phase must be one of: metadata, thumbnail, sprites, animated_thumbnails, fingerprint")
	}
	if *afterPhase == phase {
		return fmt.Errorf("after_phase cannot be the same as phase")
	}
	return nil
}

// ValidateJobMode validates a bulk job mode
func ValidateJobMode(mode string) error {
	if !JobModes[mode] {
		return fmt.Errorf("mode must be one of: missing, all")
	}
	return nil
}

// ValidateForceTarget validates a force target value
func ValidateForceTarget(forceTarget string) error {
	if !ForceTargets[forceTarget] {
		return fmt.Errorf("force_target must be one of: markers, previews, both")
	}
	return nil
}
