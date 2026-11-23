//go:build !windows

package procjob

// KillChildrenOnExit is a no-op outside Windows: on Linux the computations start ffmpeg with
// a parent-death signal (goonhub/pkg/compute, pkg/dhash) and a service stop kills the whole
// systemd cgroup; on macOS a killed worker's ffmpeg processes end with their current ffmpeg
// run (a graceful stop always kills them).
func KillChildrenOnExit() error { return nil }
