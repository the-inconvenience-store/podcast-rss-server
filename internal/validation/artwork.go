package validation

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
)

type ImageInfo struct {
	Width       int
	Height      int
	ContentType string
}

func ValidateArtwork(r io.Reader, declaredContentType string) (ImageInfo, error) {
	var header bytes.Buffer
	limitedHeader := io.LimitReader(r, 512)
	if _, err := io.Copy(&header, limitedHeader); err != nil {
		return ImageInfo{}, fmt.Errorf("read image header: %w", err)
	}
	actualContentType := http.DetectContentType(header.Bytes())
	if declaredContentType != actualContentType {
		return ImageInfo{}, fmt.Errorf("content type mismatch")
	}
	if actualContentType != "image/jpeg" && actualContentType != "image/png" {
		return ImageInfo{}, fmt.Errorf("unsupported image type")
	}

	cfg, _, err := image.DecodeConfig(io.MultiReader(bytes.NewReader(header.Bytes()), r))
	if err != nil {
		return ImageInfo{}, fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width != cfg.Height {
		return ImageInfo{}, fmt.Errorf("artwork must be square")
	}
	if cfg.Width < 1400 || cfg.Width > 3000 {
		return ImageInfo{}, fmt.Errorf("artwork size must be between 1400 and 3000 pixels")
	}
	return ImageInfo{Width: cfg.Width, Height: cfg.Height, ContentType: actualContentType}, nil
}
