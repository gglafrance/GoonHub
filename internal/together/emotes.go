package together

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// emoteNamePattern is the format of an emote name (used as :name: in the chat).
var emoteNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{2,32}$`)

// EmoteFilePattern is the format of a stored emote file name (served at /emotes/:file).
var EmoteFilePattern = regexp.MustCompile(`^[a-f0-9]{32}\.(png|gif|webp)$`)

// Emote is an emote as sent to the clients.
type EmoteDTO struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Animated bool   `json:"animated"`
	Size     int    `json:"size"`
}

func emoteDTO(e data.Emote) EmoteDTO {
	return EmoteDTO{ID: e.ID, Name: e.Name, URL: "/emotes/" + e.Filename, Width: e.Width, Height: e.Height, Animated: e.Animated, Size: e.SizeBytes}
}

// EmoteService manages the server custom emotes: images in goon2gether.emote_dir (GoonHub
// data, never a media directory), rows in the emotes table.
type EmoteService struct {
	repo     data.EmoteRepository
	cfg      config.Goon2getherConfig
	logger   *zap.Logger
	onChange func()

	mu     sync.RWMutex
	names  map[string]bool // lower-case names
	loaded bool
}

// NewEmoteService creates the emote service.
func NewEmoteService(repo data.EmoteRepository, cfg config.Goon2getherConfig, logger *zap.Logger) *EmoteService {
	return &EmoteService{repo: repo, cfg: cfg, logger: logger.With(zap.String("component", "emotes"))}
}

// OnChange registers a callback run after every change (the hub tells the lobbies).
func (s *EmoteService) OnChange(fn func()) { s.onChange = fn }

// Dir is the emote image directory.
func (s *EmoteService) Dir() string { return s.cfg.EmoteDir }

func (s *EmoteService) changed() {
	s.mu.Lock()
	s.loaded = false
	s.mu.Unlock()
	if s.onChange != nil {
		s.onChange()
	}
}

// HasEmote tells whether an emote has this name (case-insensitive).
func (s *EmoteService) HasEmote(name string) bool {
	s.mu.RLock()
	if s.loaded {
		ok := s.names[strings.ToLower(name)]
		s.mu.RUnlock()
		return ok
	}
	s.mu.RUnlock()
	emotes, err := s.repo.List()
	if err != nil {
		s.logger.Warn("Failed to load the emotes", zap.Error(err))
		return false
	}
	names := make(map[string]bool, len(emotes))
	for _, e := range emotes {
		names[strings.ToLower(e.Name)] = true
	}
	s.mu.Lock()
	s.names, s.loaded = names, true
	s.mu.Unlock()
	return names[strings.ToLower(name)]
}

// List gives every emote, by name.
func (s *EmoteService) List() ([]EmoteDTO, error) {
	emotes, err := s.repo.List()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list the emotes", err)
	}
	out := make([]EmoteDTO, len(emotes))
	for i, e := range emotes {
		out[i] = emoteDTO(e)
	}
	return out, nil
}

func validEmoteName(name string) error {
	if !emoteNamePattern.MatchString(name) {
		return apperrors.NewValidationErrorWithField("name", "name must be 2 to 32 letters, digits or underscores")
	}
	return nil
}

// emoteImage is what a file inspection found.
type emoteImage struct {
	contentType string
	ext         string
	width       int
	height      int
	animated    bool
}

// inspectEmote checks the type, the dimensions and the animation of an emote file.
func inspectEmote(b []byte) (*emoteImage, error) {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		cfg, err := png.DecodeConfig(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("invalid PNG: %w", err)
		}
		return &emoteImage{contentType: "image/png", ext: "png", width: cfg.Width, height: cfg.Height, animated: pngAnimated(b)}, nil
	case bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a")):
		g, err := gif.DecodeAll(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("invalid GIF: %w", err)
		}
		return &emoteImage{contentType: "image/gif", ext: "gif", width: g.Config.Width, height: g.Config.Height, animated: len(g.Image) > 1}, nil
	case len(b) >= 16 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		w, h, animated, err := webpInfo(b)
		if err != nil {
			return nil, err
		}
		return &emoteImage{contentType: "image/webp", ext: "webp", width: w, height: h, animated: animated}, nil
	}
	return nil, errors.New("the file must be a PNG, GIF or WebP image")
}

// pngAnimated finds an APNG animation control chunk before the image data.
func pngAnimated(b []byte) bool {
	for i := 8; i+8 <= len(b); {
		n := int(binary.BigEndian.Uint32(b[i : i+4]))
		kind := string(b[i+4 : i+8])
		switch kind {
		case "acTL":
			return true
		case "IDAT", "IEND":
			return false
		}
		next := i + 12 + n
		if n < 0 || next <= i {
			return false
		}
		i = next
	}
	return false
}

// webpInfo reads the canvas size and the animation flag of a WebP file.
func webpInfo(b []byte) (width, height int, animated bool, err error) {
	if len(b) < 30 {
		return 0, 0, false, errors.New("invalid WebP: too short")
	}
	switch string(b[12:16]) {
	case "VP8X":
		flags := b[20]
		w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
		h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16
		return w + 1, h + 1, flags&0x02 != 0, nil
	case "VP8 ":
		if b[23] != 0x9d || b[24] != 0x01 || b[25] != 0x2a {
			return 0, 0, false, errors.New("invalid WebP: bad VP8 frame")
		}
		w := int(binary.LittleEndian.Uint16(b[26:28]) & 0x3fff)
		h := int(binary.LittleEndian.Uint16(b[28:30]) & 0x3fff)
		return w, h, false, nil
	case "VP8L":
		if b[20] != 0x2f {
			return 0, 0, false, errors.New("invalid WebP: bad VP8L signature")
		}
		bits := binary.LittleEndian.Uint32(b[21:25])
		return int(bits&0x3fff) + 1, int((bits>>14)&0x3fff) + 1, false, nil
	}
	return 0, 0, false, errors.New("invalid WebP: unknown format")
}

// Upload stores a new emote.
func (s *EmoteService) Upload(name string, r io.Reader, createdBy uint) (*EmoteDTO, error) {
	name = strings.TrimSpace(name)
	if err := validEmoteName(name); err != nil {
		return nil, err
	}
	if max := s.cfg.EmoteMaxCount; max > 0 {
		n, err := s.repo.Count()
		if err != nil {
			return nil, apperrors.NewInternalError("failed to count the emotes", err)
		}
		if n >= int64(max) {
			return nil, apperrors.NewValidationError(fmt.Sprintf("the server already has %d emotes", n))
		}
	}
	maxBytes := s.cfg.EmoteMaxBytes
	if maxBytes <= 0 {
		maxBytes = 512 << 10
	}
	b, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, apperrors.NewValidationError("failed to read the file")
	}
	if int64(len(b)) > maxBytes {
		return nil, apperrors.NewValidationErrorWithField("file", fmt.Sprintf("the file is larger than %d KB", maxBytes>>10))
	}
	img, err := inspectEmote(b)
	if err != nil {
		return nil, apperrors.NewValidationErrorWithField("file", err.Error())
	}
	if dim := s.cfg.EmoteMaxDimension; dim > 0 && (img.width > dim || img.height > dim) {
		return nil, apperrors.NewValidationErrorWithField("file", fmt.Sprintf("the image is larger than %dx%d pixels", dim, dim))
	}
	if img.width <= 0 || img.height <= 0 {
		return nil, apperrors.NewValidationErrorWithField("file", "the image has no size")
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, apperrors.NewInternalError("failed to name the emote file", err)
	}
	filename := hex.EncodeToString(random) + "." + img.ext
	if err := os.MkdirAll(s.cfg.EmoteDir, 0o755); err != nil {
		return nil, apperrors.NewInternalError("failed to create the emote directory", err)
	}
	path := filepath.Join(s.cfg.EmoteDir, filename)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return nil, apperrors.NewInternalError("failed to store the emote", err)
	}
	e := &data.Emote{
		Name:        name,
		Filename:    filename,
		ContentType: img.contentType,
		Width:       img.width,
		Height:      img.height,
		SizeBytes:   len(b),
		Animated:    img.animated,
		CreatedBy:   &createdBy,
	}
	if err := s.repo.Create(e); err != nil {
		if rmErr := os.Remove(path); rmErr != nil {
			s.logger.Warn("Failed to remove an emote file", zap.String("file", filename), zap.Error(rmErr))
		}
		if errors.Is(err, data.ErrEmoteNameTaken) {
			return nil, apperrors.NewConflictError("emote", "an emote already has this name")
		}
		return nil, apperrors.NewInternalError("failed to save the emote", err)
	}
	s.changed()
	dto := emoteDTO(*e)
	return &dto, nil
}

// Rename changes the name of an emote.
func (s *EmoteService) Rename(id uint, name string) (*EmoteDTO, error) {
	name = strings.TrimSpace(name)
	if err := validEmoteName(name); err != nil {
		return nil, err
	}
	if err := s.repo.Rename(id, name); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, apperrors.NewNotFoundError("emote", id)
		case errors.Is(err, data.ErrEmoteNameTaken):
			return nil, apperrors.NewConflictError("emote", "an emote already has this name")
		}
		return nil, apperrors.NewInternalError("failed to rename the emote", err)
	}
	e, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to read the emote", err)
	}
	s.changed()
	dto := emoteDTO(*e)
	return &dto, nil
}

// Delete removes an emote and its image file (an emote image, never a media file).
func (s *EmoteService) Delete(id uint) error {
	e, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.NewNotFoundError("emote", id)
		}
		return apperrors.NewInternalError("failed to read the emote", err)
	}
	if err := s.repo.Delete(id); err != nil {
		return apperrors.NewInternalError("failed to delete the emote", err)
	}
	if EmoteFilePattern.MatchString(e.Filename) {
		if err := os.Remove(filepath.Join(s.cfg.EmoteDir, e.Filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Warn("Failed to remove an emote file", zap.String("file", e.Filename), zap.Error(err))
		}
	}
	s.changed()
	return nil
}
