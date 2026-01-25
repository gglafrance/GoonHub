package validators

import "fmt"

// TriggerConfig represents a trigger configuration for cycle detection
type TriggerConfig struct {
	Phase       string
	TriggerType string
	AfterPhase  *string
}

// DetectTriggerCycle checks if adding a new after_job dependency would create a cycle
func DetectTriggerCycle(configs []TriggerConfig, newPhase, newAfterPhase string) error {
	// Build adjacency: phase -> after_phase (what this phase depends on)
	dependsOn := make(map[string]string)
	for _, cfg := range configs {
		if cfg.TriggerType == "after_job" && cfg.AfterPhase != nil {
			dependsOn[cfg.Phase] = *cfg.AfterPhase
		}
	}

	// Apply the proposed change
	dependsOn[newPhase] = newAfterPhase

	// Walk from phase following the chain to detect a cycle
	visited := make(map[string]bool)
	current := newPhase
	for {
		if visited[current] {
			return fmt.Errorf("circular dependency detected: %s would create a cycle", newPhase)
		}
		visited[current] = true
		next, exists := dependsOn[current]
		if !exists {
			break
		}
		current = next
	}
	return nil
}
