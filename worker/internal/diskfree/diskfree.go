// Package diskfree reports the free space of the file system holding a path.
package diskfree

// Free returns the bytes available to the current user on the file system of path.
func Free(path string) (int64, error) { return free(path) }
