// Package procjob makes the child processes of the worker (ffmpeg, ffprobe, fpcalc) die with
// it, even when the worker is killed (Task Manager, taskkill /F, a crash): an orphaned ffmpeg
// would keep the CPU/GPU busy for a job the server has already handed to someone else, and
// keep its output files locked.
package procjob

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// KillChildrenOnExit puts the worker in a Job Object with KILL_ON_JOB_CLOSE. Child processes
// join the job when they start; the job handle is never closed by the worker, so Windows
// closes it when the worker exits, however it exits, and kills every process left in it.
func KillChildrenOnExit() error {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("CreateJobObject: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(h)
		return fmt.Errorf("SetInformationJobObject: %w", err)
	}
	if err := windows.AssignProcessToJobObject(h, windows.CurrentProcess()); err != nil {
		_ = windows.CloseHandle(h)
		return fmt.Errorf("AssignProcessToJobObject: %w", err)
	}
	return nil
}
