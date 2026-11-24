package workerproto

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// MaxArtifactNameLen is the longest artifact name accepted.
const MaxArtifactNameLen = 128

// ErrInvalidArtifactName is returned (wrapped) by ValidateArtifactName.
var ErrInvalidArtifactName = errors.New("invalid artifact name")

// windowsReserved are device names Windows refuses as file names (with any extension).
var windowsReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// ValidateArtifactName checks that name is a plain file name, safe to join to a directory
// on any OS: 1 to MaxArtifactNameLen characters among [A-Za-z0-9._-], no leading dot (no
// hidden file, no "." or ".."), no "..", no trailing dot, no Windows device name. So no
// path separator, no drive letter, no traversal.
func ValidateArtifactName(name string) error {
	if name == "" || len(name) > MaxArtifactNameLen {
		return fmt.Errorf("%w: %q (length must be 1-%d)", ErrInvalidArtifactName, name, MaxArtifactNameLen)
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-'
		if !ok {
			return fmt.Errorf("%w: %q (character %q not allowed)", ErrInvalidArtifactName, name, c)
		}
	}
	if name[0] == '.' || name[len(name)-1] == '.' || strings.Contains(name, "..") {
		return fmt.Errorf("%w: %q (leading, trailing or double dot)", ErrInvalidArtifactName, name)
	}
	base := strings.ToUpper(name)
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	if windowsReserved[base] {
		return fmt.Errorf("%w: %q (reserved device name)", ErrInvalidArtifactName, name)
	}
	return nil
}

// ValidateArtifacts checks a complete's artifact list: valid unique names, non-negative
// sizes, well-formed sha256, and every file referenced by result declared.
func ValidateArtifacts(refs []ArtifactRef, result PhaseResult) error {
	declared := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if err := ValidateArtifactName(ref.Name); err != nil {
			return err
		}
		if declared[ref.Name] {
			return fmt.Errorf("artifact %q declared twice", ref.Name)
		}
		declared[ref.Name] = true
		if ref.Size < 0 {
			return fmt.Errorf("artifact %q: negative size", ref.Name)
		}
		if !ValidSHA256(ref.SHA256) {
			return fmt.Errorf("artifact %q: sha256 must be 64 lowercase hex characters", ref.Name)
		}
	}
	for _, f := range result.Files() {
		if !declared[f] {
			return fmt.Errorf("result references undeclared artifact %q", f)
		}
	}
	return nil
}

// ValidSHA256 reports whether s is a lowercase hex sha256.
func ValidSHA256(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// SHA256File returns the lowercase hex sha256 and the size of a file.
func SHA256File(path string) (sum string, size int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer f.Close()
	return SHA256Reader(f)
}

// SHA256Reader returns the lowercase hex sha256 and the byte count of everything read.
func SHA256Reader(r io.Reader) (sum string, size int64, err error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", n, fmt.Errorf("failed to hash: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// ArtifactRefForFile builds the ArtifactRef of a file (name = base name given).
func ArtifactRefForFile(name, path string) (ArtifactRef, error) {
	if err := ValidateArtifactName(name); err != nil {
		return ArtifactRef{}, err
	}
	sum, size, err := SHA256File(path)
	if err != nil {
		return ArtifactRef{}, err
	}
	return ArtifactRef{Name: name, Size: size, SHA256: sum}, nil
}

// ---------------------------------------------------------------------------------------
// Fingerprint encoding: base64 of little-endian bytes
// ---------------------------------------------------------------------------------------

// AudioFingerprint is a chromaprint raw fingerprint; JSON: base64 of int32 LE bytes.
type AudioFingerprint []int32

// VisualFingerprint is a list of 64-bit dHashes; JSON: base64 of uint64 LE bytes.
type VisualFingerprint []uint64

// Bytes returns the little-endian bytes (the PostgreSQL storage format).
func (a AudioFingerprint) Bytes() []byte {
	buf := make([]byte, len(a)*4)
	for i, v := range a {
		binary.LittleEndian.PutUint32(buf[i*4:], uint32(v))
	}
	return buf
}

// AudioFingerprintFromBytes decodes little-endian int32 bytes.
func AudioFingerprintFromBytes(b []byte) (AudioFingerprint, error) {
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("audio fingerprint: %d bytes is not a multiple of 4", len(b))
	}
	out := make(AudioFingerprint, len(b)/4)
	for i := range out {
		out[i] = int32(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out, nil
}

// MarshalJSON encodes as a base64 string.
func (a AudioFingerprint) MarshalJSON() ([]byte, error) {
	return json.Marshal(base64.StdEncoding.EncodeToString(a.Bytes()))
}

// UnmarshalJSON decodes a base64 string.
func (a *AudioFingerprint) UnmarshalJSON(data []byte) error {
	b, err := decodeBase64JSON(data)
	if err != nil {
		return fmt.Errorf("audio fingerprint: %w", err)
	}
	if b == nil {
		*a = nil
		return nil
	}
	v, err := AudioFingerprintFromBytes(b)
	if err != nil {
		return err
	}
	*a = v
	return nil
}

// Bytes returns the little-endian bytes (the PostgreSQL storage format).
func (v VisualFingerprint) Bytes() []byte {
	buf := make([]byte, len(v)*8)
	for i, h := range v {
		binary.LittleEndian.PutUint64(buf[i*8:], h)
	}
	return buf
}

// VisualFingerprintFromBytes decodes little-endian uint64 bytes.
func VisualFingerprintFromBytes(b []byte) (VisualFingerprint, error) {
	if len(b)%8 != 0 {
		return nil, fmt.Errorf("visual fingerprint: %d bytes is not a multiple of 8", len(b))
	}
	out := make(VisualFingerprint, len(b)/8)
	for i := range out {
		out[i] = binary.LittleEndian.Uint64(b[i*8:])
	}
	return out, nil
}

// MarshalJSON encodes as a base64 string.
func (v VisualFingerprint) MarshalJSON() ([]byte, error) {
	return json.Marshal(base64.StdEncoding.EncodeToString(v.Bytes()))
}

// UnmarshalJSON decodes a base64 string.
func (v *VisualFingerprint) UnmarshalJSON(data []byte) error {
	b, err := decodeBase64JSON(data)
	if err != nil {
		return fmt.Errorf("visual fingerprint: %w", err)
	}
	if b == nil {
		*v = nil
		return nil
	}
	out, err := VisualFingerprintFromBytes(b)
	if err != nil {
		return err
	}
	*v = out
	return nil
}

func decodeBase64JSON(data []byte) ([]byte, error) {
	if string(data) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("expected a base64 string: %w", err)
	}
	if s == "" {
		return nil, nil
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	return b, nil
}
