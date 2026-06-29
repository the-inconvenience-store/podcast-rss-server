package validation_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/samstevens/podcast-rss/internal/validation"
)

func TestValidateArtworkAcceptsJPEGAndPNGSquareWithinAppleBounds(t *testing.T) {
	for _, tc := range []struct {
		name        string
		contentType string
		data        []byte
	}{
		{name: "1400 png", contentType: "image/png", data: pngImage(t, 1400, 1400)},
		{name: "3000 jpeg", contentType: "image/jpeg", data: jpegImage(t, 3000, 3000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info, err := validation.ValidateArtwork(bytes.NewReader(tc.data), tc.contentType)
			if err != nil {
				t.Fatalf("ValidateArtwork returned error: %v", err)
			}
			if info.Width != info.Height {
				t.Fatalf("image was not square: %+v", info)
			}
		})
	}
}

func TestValidateArtworkRejectsInvalidImages(t *testing.T) {
	for _, tc := range []struct {
		name        string
		contentType string
		data        []byte
	}{
		{name: "too small", contentType: "image/png", data: pngImage(t, 1399, 1399)},
		{name: "not square", contentType: "image/png", data: pngImage(t, 1400, 1500)},
		{name: "gif", contentType: "image/gif", data: []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")},
		{name: "mismatched content type", contentType: "image/png", data: jpegImage(t, 1400, 1400)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := validation.ValidateArtwork(bytes.NewReader(tc.data), tc.contentType); err == nil {
				t.Fatal("ValidateArtwork returned nil error")
			}
		})
	}
}

func pngImage(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func jpegImage(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{B: 255, A: 255})
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}
