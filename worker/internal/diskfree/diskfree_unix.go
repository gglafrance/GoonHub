//go:build !windows

package diskfree

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func free(path string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	return int64(st.Bavail) * int64(st.Bsize), nil //nolint:unconvert // field types differ per OS
}
