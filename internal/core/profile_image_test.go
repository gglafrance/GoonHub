package core

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"goonhub/internal/apperrors"
)

var testImageLimits = ProfileImageLimits{MaxBytes: 4 << 20, MaxPixels: 4_000_000}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func solidImage(w, h int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// gifHeader is a GIF whose header claims w x h (only the header: DecodeConfig is enough).
func gifHeader(w, h uint16) []byte {
	b := []byte("GIF89a")
	b = binary.LittleEndian.AppendUint16(b, w)
	b = binary.LittleEndian.AppendUint16(b, h)
	return append(b, 0, 0, 0)
}

func TestProcessProfileImageSizes(t *testing.T) {
	src := encodePNG(t, solidImage(900, 300, color.NRGBA{R: 200, G: 10, B: 10, A: 255}))
	out, err := ProcessProfileImage(bytes.NewReader(src), testImageLimits, ProfileImageCrop{}, ProfileAvatarSizes)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range ProfileAvatarSizes {
		img, err := jpeg.Decode(bytes.NewReader(out[size.Name]))
		if err != nil {
			t.Fatalf("%s: not a JPEG: %v", size.Name, err)
		}
		if b := img.Bounds(); b.Dx() != size.Width || b.Dy() != size.Height {
			t.Fatalf("%s: %v, want %dx%d", size.Name, b, size.Width, size.Height)
		}
		r, g, _, _ := img.At(size.Width/2, size.Height/2).RGBA()
		if r>>8 < 180 || g>>8 > 40 {
			t.Fatalf("%s: colour lost: %d %d", size.Name, r>>8, g>>8)
		}
	}
	banner, err := ProcessProfileImage(bytes.NewReader(src), testImageLimits, ProfileImageCrop{}, ProfileBannerSizes)
	if err != nil || len(banner) != len(ProfileBannerSizes) {
		t.Fatalf("banner: %v %v", len(banner), err)
	}
}

func TestProcessProfileImageRejects(t *testing.T) {
	cases := []struct {
		name   string
		input  []byte
		limits ProfileImageLimits
		want   string
	}{
		{"text", []byte("<?php echo 1; ?>"), testImageLimits, "not a JPEG"},
		{"empty", nil, testImageLimits, "empty"},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"></svg>`), testImageLimits, "not a JPEG"},
		{"too many bytes", encodePNG(t, solidImage(64, 64, color.White)), ProfileImageLimits{MaxBytes: 50, MaxPixels: 1 << 20}, "larger than"},
		{"header claims a huge image", gifHeader(30000, 30000), testImageLimits, "too large"},
		{"over the pixel limit", gifHeader(2001, 2000), testImageLimits, "too large"},
		{"too small", encodePNG(t, solidImage(8, 8, color.White)), testImageLimits, "at least"},
		{"truncated", encodePNG(t, solidImage(64, 64, color.White))[:40], testImageLimits, "could not be decoded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ProcessProfileImage(bytes.NewReader(tc.input), tc.limits, ProfileImageCrop{}, ProfileAvatarSizes)
			if err == nil || !apperrors.IsValidation(err) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want a validation error containing %q", err, tc.want)
			}
		})
	}
}

func TestCropRect(t *testing.T) {
	bounds := image.Rect(0, 0, 1000, 500)
	cases := []struct {
		name   string
		crop   ProfileImageCrop
		aw, ah int
		want   image.Rectangle
	}{
		{"center square", ProfileImageCrop{}, 1, 1, image.Rect(250, 0, 750, 500)},
		{"banner from landscape", ProfileImageCrop{}, 3, 1, image.Rect(0, 83, 1000, 416)},
		{"requested square", ProfileImageCrop{X: 100, Y: 50, W: 200, H: 200}, 1, 1, image.Rect(100, 50, 300, 250)},
		{"requested crop clamped", ProfileImageCrop{X: 900, Y: 0, W: 400, H: 400}, 1, 1, image.Rect(900, 150, 1000, 250)},
		{"requested crop too small: whole image", ProfileImageCrop{X: 10, Y: 10, W: 5, H: 5}, 1, 1, image.Rect(250, 0, 750, 500)},
	}
	for _, tc := range cases {
		if got := cropRect(bounds, tc.crop, tc.aw, tc.ah); got != tc.want {
			t.Fatalf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestResizeAreaAverages(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i, v := range []uint8{0, 255, 255, 0} {
		src.Set(i%2, i/2, color.RGBA{R: v, G: v, B: v, A: 255})
	}
	out := resizeArea(src, 1, 1)
	if r := out.Pix[0]; r < 126 || r > 129 {
		t.Fatalf("average of black and white = %d", r)
	}
	up := resizeArea(src, 4, 4)
	if up.Pix[0] != 0 || up.Pix[up.PixOffset(3, 0)] != 255 {
		t.Fatalf("upscale: %v", up.Pix[:16])
	}
}

func TestProcessProfileImageTransparency(t *testing.T) {
	src := encodePNG(t, solidImage(64, 64, color.NRGBA{}))
	out, err := ProcessProfileImage(bytes.NewReader(src), testImageLimits, ProfileImageCrop{}, ProfileAvatarSizes[:1])
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out["sm"]))
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := img.At(10, 10).RGBA(); r>>8 > 30 {
		t.Fatalf("transparent pixels not on the dark background: %d", r>>8)
	}
}
