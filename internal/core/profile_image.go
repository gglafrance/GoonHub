package core

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // registers the GIF decoder (first frame)
	"image/jpeg"
	_ "image/png" // registers the PNG decoder
	"io"

	"goonhub/internal/apperrors"
)

// Profile images (avatars and banners, docs/PROFILES.md): the upload is checked (size,
// format, dimensions read from the header before anything is decoded), cropped to the target
// aspect ratio, resized to a few sizes with an area-averaging filter and re-encoded as JPEG.
// Re-encoding drops whatever the upload carried besides the pixels (metadata, other frames).

// ProfileImageSize is an output size of a profile image.
type ProfileImageSize struct {
	Name   string
	Width  int
	Height int
}

// Avatar sizes (square) and banner sizes (3:1).
var (
	ProfileAvatarSizes = []ProfileImageSize{{"sm", 64, 64}, {"md", 160, 160}, {"lg", 400, 400}}
	ProfileBannerSizes = []ProfileImageSize{{"md", 750, 250}, {"lg", 1500, 500}}
)

// ProfileImageCrop is the part of the uploaded image to keep, in source pixels. It is
// clamped to the image and then reduced to the target aspect ratio around its center. A zero
// crop keeps the whole image (center crop).
type ProfileImageCrop struct {
	X, Y, W, H int
}

// ProfileImageLimits bound what an upload may be.
type ProfileImageLimits struct {
	MaxBytes  int64
	MaxPixels int
}

const (
	profileImageMinSide   = 16
	profileImageMaxSide   = 20000
	profileImageJPEGQual  = 88
	profileImageFormatGIF = "gif"
)

// profileImageBackground fills the transparent parts (JPEG has no alpha): the app background.
var profileImageBackground = color.RGBA{R: 0x0F, G: 0x0F, B: 0x0F, A: 0xFF}

// ErrProfileImageTooLarge: the upload is bigger than allowed (bytes or pixels).
func ErrProfileImageTooLarge(msg string) error {
	return apperrors.NewValidationErrorWithField("image", msg)
}

// readProfileImage reads the upload, refusing more than maxBytes.
func readProfileImage(r io.Reader, maxBytes int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read the image: %w", err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, ErrProfileImageTooLarge(fmt.Sprintf("the image is larger than %d bytes", maxBytes))
	}
	if len(raw) == 0 {
		return nil, apperrors.NewValidationErrorWithField("image", "the image is empty")
	}
	return raw, nil
}

// decodeProfileImage checks the header (format, dimensions) and only then decodes the image.
func decodeProfileImage(raw []byte, limits ProfileImageLimits) (image.Image, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, apperrors.NewValidationErrorWithField("image", "not a JPEG, PNG or GIF image")
	}
	switch format {
	case "jpeg", "png", profileImageFormatGIF:
	default:
		return nil, apperrors.NewValidationErrorWithField("image", "not a JPEG, PNG or GIF image")
	}
	if cfg.Width < profileImageMinSide || cfg.Height < profileImageMinSide {
		return nil, apperrors.NewValidationErrorWithField("image", fmt.Sprintf("the image must be at least %dx%d pixels", profileImageMinSide, profileImageMinSide))
	}
	if cfg.Width > profileImageMaxSide || cfg.Height > profileImageMaxSide ||
		int64(cfg.Width)*int64(cfg.Height) > int64(limits.MaxPixels) {
		return nil, ErrProfileImageTooLarge(fmt.Sprintf("the image is too large (%dx%d, at most %d pixels)", cfg.Width, cfg.Height, limits.MaxPixels))
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, apperrors.NewValidationErrorWithField("image", "the image could not be decoded")
	}
	return img, nil
}

// cropRect returns the part of bounds to keep: the requested crop clamped to the image, then
// the largest rectangle of the aspect ratio aw:ah centered in it.
func cropRect(bounds image.Rectangle, crop ProfileImageCrop, aw, ah int) image.Rectangle {
	r := bounds
	if crop.W > 0 && crop.H > 0 {
		req := image.Rect(bounds.Min.X+crop.X, bounds.Min.Y+crop.Y, bounds.Min.X+crop.X+crop.W, bounds.Min.Y+crop.Y+crop.H).Intersect(bounds)
		if req.Dx() >= profileImageMinSide && req.Dy() >= profileImageMinSide {
			r = req
		}
	}
	w, h := r.Dx(), r.Dy()
	// Largest aw:ah rectangle inside w x h
	if w*ah > h*aw {
		w = h * aw / ah
	} else {
		h = w * ah / aw
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	x := r.Min.X + (r.Dx()-w)/2
	y := r.Min.Y + (r.Dy()-h)/2
	return image.Rect(x, y, x+w, y+h)
}

// toRGBA copies a part of an image onto an opaque RGBA image (transparency over the app
// background).
func toRGBA(img image.Image, r image.Rectangle) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: profileImageBackground}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Over)
	return dst
}

// resizeArea resizes an RGBA image to w x h by averaging, for each output pixel, the source
// pixels it covers (weighted by coverage): a sharp and alias-free downscale. Upscaling falls
// back to the nearest pixels.
func resizeArea(src *image.RGBA, w, h int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sx := float64(sw) / float64(w)
	sy := float64(sh) / float64(h)
	for y := 0; y < h; y++ {
		y0 := float64(y) * sy
		y1 := y0 + sy
		for x := 0; x < w; x++ {
			x0 := float64(x) * sx
			x1 := x0 + sx
			var r, g, b, a, total float64
			for py := int(y0); py < sh && float64(py) < y1; py++ {
				wy := minf(float64(py+1), y1) - maxf(float64(py), y0)
				if wy <= 0 {
					continue
				}
				row := src.Pix[py*src.Stride:]
				for px := int(x0); px < sw && float64(px) < x1; px++ {
					wx := minf(float64(px+1), x1) - maxf(float64(px), x0)
					if wx <= 0 {
						continue
					}
					wgt := wx * wy
					i := px * 4
					r += float64(row[i]) * wgt
					g += float64(row[i+1]) * wgt
					b += float64(row[i+2]) * wgt
					a += float64(row[i+3]) * wgt
					total += wgt
				}
			}
			o := dst.PixOffset(x, y)
			if total > 0 {
				dst.Pix[o] = uint8(r/total + 0.5)
				dst.Pix[o+1] = uint8(g/total + 0.5)
				dst.Pix[o+2] = uint8(b/total + 0.5)
				dst.Pix[o+3] = uint8(a/total + 0.5)
			}
		}
	}
	return dst
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ProcessProfileImage turns an upload into JPEG files of the given sizes (all of the same
// aspect ratio), keyed by size name. Errors are validation errors (400) for bad uploads.
func ProcessProfileImage(r io.Reader, limits ProfileImageLimits, crop ProfileImageCrop, sizes []ProfileImageSize) (map[string][]byte, error) {
	if len(sizes) == 0 {
		return nil, fmt.Errorf("no output size")
	}
	raw, err := readProfileImage(r, limits.MaxBytes)
	if err != nil {
		return nil, err
	}
	img, err := decodeProfileImage(raw, limits)
	if err != nil {
		return nil, err
	}
	rect := cropRect(img.Bounds(), crop, sizes[0].Width, sizes[0].Height)
	src := toRGBA(img, rect)
	out := make(map[string][]byte, len(sizes))
	for _, size := range sizes {
		resized := resizeArea(src, size.Width, size.Height)
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: profileImageJPEGQual}); err != nil {
			return nil, fmt.Errorf("failed to encode the image: %w", err)
		}
		out[size.Name] = buf.Bytes()
	}
	return out, nil
}
