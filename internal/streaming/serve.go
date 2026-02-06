package streaming

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ServeVideo serves video content with efficient range request handling using a
// provided buffer for io.CopyBuffer. It handles single byte-range requests directly
// (the 99%+ case for video streaming) and falls back to http.ServeContent for
// multipart ranges or other edge cases.
func ServeVideo(w http.ResponseWriter, r *http.Request, name string, modtime time.Time, content io.ReadSeeker, buf []byte) {
	// Determine content size
	size, err := content.Seek(0, io.SeekEnd)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if _, err := content.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Handle If-Modified-Since for 304 responses
	if !modtime.IsZero() {
		w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
		if ims := r.Header.Get("If-Modified-Since"); ims != "" {
			if t, parseErr := http.ParseTime(ims); parseErr == nil {
				if modtime.Before(t.Add(1 * time.Second)) {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
		}
	}

	w.Header().Set("Accept-Ranges", "bytes")

	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		// No range requested — serve entire file
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			io.CopyBuffer(w, content, buf) //nolint:errcheck
		}
		return
	}

	// Try to parse as a single byte range (covers 99%+ of video requests)
	start, length, ok := parseSingleRange(rangeHeader, size)
	if !ok {
		// Multipart range or malformed — delegate to stdlib
		if _, err := content.Seek(0, io.SeekStart); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, name, modtime, content)
		return
	}

	if _, err := content.Seek(start, io.SeekStart); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+length-1, size))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(http.StatusPartialContent)

	if r.Method != http.MethodHead {
		io.CopyBuffer(w, io.LimitReader(content, length), buf) //nolint:errcheck
	}
}

// parseSingleRange parses a Range header containing exactly one byte range.
// Returns (start, length, true) on success, or (0, 0, false) for multipart
// ranges, invalid syntax, or unsatisfiable ranges.
//
// Supported formats:
//   - bytes=start-end
//   - bytes=start-      (from start to EOF)
//   - bytes=-suffix     (last N bytes)
func parseSingleRange(rangeHeader string, size int64) (start, length int64, ok bool) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, false
	}
	spec := strings.TrimPrefix(rangeHeader, "bytes=")

	// Reject multipart ranges
	if strings.Contains(spec, ",") {
		return 0, 0, false
	}

	spec = strings.TrimSpace(spec)
	dashIdx := strings.IndexByte(spec, '-')
	if dashIdx < 0 {
		return 0, 0, false
	}

	startStr := strings.TrimSpace(spec[:dashIdx])
	endStr := strings.TrimSpace(spec[dashIdx+1:])

	if startStr == "" {
		// Suffix range: bytes=-N (last N bytes)
		suffix, parseErr := strconv.ParseInt(endStr, 10, 64)
		if parseErr != nil || suffix <= 0 {
			return 0, 0, false
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, suffix, true
	}

	s, parseErr := strconv.ParseInt(startStr, 10, 64)
	if parseErr != nil || s < 0 || s >= size {
		return 0, 0, false
	}

	if endStr == "" {
		// Open range: bytes=N- (from N to EOF)
		return s, size - s, true
	}

	e, parseErr := strconv.ParseInt(endStr, 10, 64)
	if parseErr != nil || e < s {
		return 0, 0, false
	}
	if e >= size {
		e = size - 1
	}

	return s, e - s + 1, true
}
