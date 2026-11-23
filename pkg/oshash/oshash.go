// Package oshash computes the OpenSubtitles hash of a file, as Stash does: the 64-bit sum of
// the file size and of the little-endian uint64 words of the first and the last 64 KiB of the
// file, written as 16 hexadecimal digits. Only 128 KiB are read, whatever the file size, so it
// is a cheap way to recognise the same file at two paths (with the size).
package oshash

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// chunkSize is the size of the head and of the tail that are hashed.
const chunkSize int64 = 64 * 1024

// ErrEmptyFile is returned for an empty file: it has no content to compare (Stash gives it an
// empty hash), so it never matches another file.
var ErrEmptyFile = errors.New("oshash: empty file")

// openFile opens the file to hash. It is a variable so that the tests can check the flags.
var openFile = func(name string, flag int, perm fs.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

// FromFile returns the oshash of the file at path. The file is opened read-only and only its
// head and tail are read: it is never written.
func FromFile(path string) (string, error) {
	f, err := openFile(path, os.O_RDONLY, 0)
	if err != nil {
		return "", fmt.Errorf("oshash: open %s: %w", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("oshash: stat %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("oshash: %s is not a regular file", path)
	}
	return FromReaderAt(f, info.Size())
}

// FromReaderAt returns the oshash of content of the given size. A file smaller than 64 KiB is
// hashed like Stash does: head and tail are its first and last (size / 8) * 8 bytes, which
// overlap.
func FromReaderAt(r io.ReaderAt, size int64) (string, error) {
	if size <= 0 {
		return "", ErrEmptyFile
	}
	chunk := chunkSize
	if size < chunk {
		chunk = (size / 8) * 8 // must be a multiple of 8
	}
	buf := make([]byte, chunk)
	sum := uint64(size)

	if chunk > 0 {
		if err := readFull(r, buf, 0); err != nil {
			return "", fmt.Errorf("oshash: read head: %w", err)
		}
		sum += sumWords(buf)
		if err := readFull(r, buf, size-chunk); err != nil {
			return "", fmt.Errorf("oshash: read tail: %w", err)
		}
		sum += sumWords(buf)
	}
	return fmt.Sprintf("%016x", sum), nil
}

// readFull fills buf from offset off. ReaderAt may return io.EOF with a full buffer when the
// read ends at the end of the file: that is not an error.
func readFull(r io.ReaderAt, buf []byte, off int64) error {
	n, err := r.ReadAt(buf, off)
	if n == len(buf) {
		return nil
	}
	if err == nil || errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

// sumWords adds the little-endian uint64 words of buf (its length is a multiple of 8),
// wrapping around on overflow.
func sumWords(buf []byte) uint64 {
	var sum uint64
	for i := 0; i+8 <= len(buf); i += 8 {
		sum += binary.LittleEndian.Uint64(buf[i : i+8])
	}
	return sum
}
