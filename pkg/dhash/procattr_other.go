//go:build !linux

package dhash

import "os/exec"

// dieWithParent is a no-op outside Linux (the Windows worker runs in a kill-on-close Job
// Object, which covers its children).
func dieWithParent(cmd *exec.Cmd) {}
