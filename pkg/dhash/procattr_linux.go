package dhash

import (
	"os/exec"
	"syscall"
)

// dieWithParent makes the child process receive SIGKILL when the process that started it
// dies (a killed remote worker must not leave its ffmpeg running for a job the server has
// already handed to someone else).
func dieWithParent(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
