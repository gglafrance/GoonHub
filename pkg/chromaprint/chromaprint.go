package chromaprint

import (
	"fmt"
	"os/exec"
)

// CheckInstallation verifies that fpcalc is available on PATH
func CheckInstallation() error {
	_, err := exec.LookPath("fpcalc")
	if err != nil {
		return fmt.Errorf("fpcalc not found on PATH: %w", err)
	}
	return nil
}

// FpcalcPath returns the path to the fpcalc binary
func FpcalcPath() string {
	path, err := exec.LookPath("fpcalc")
	if err != nil {
		return "fpcalc"
	}
	return path
}
