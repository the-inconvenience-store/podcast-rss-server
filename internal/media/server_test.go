package media_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/samstevens/podcast-rss/internal/media"
	"github.com/samstevens/podcast-rss/internal/storage"
)

func TestHandlerServesMediaWithRangeSupportAndLengthAndType(t *testing.T) {
	store := storage.NewMemory()
	content := []byte("0123456789abcdef")
	if err := store.Put("media/show-1/episode-1/audio.mp3", bytes.NewReader(content), storage.ObjectInfo{
		Size:        int64(len(content)),
		ContentType: "audio/mpeg",
		ModTime:     time.Unix(100, 0).UTC(),
	}); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	handler := media.Handler(store)

	full := httptest.NewRecorder()
	handler.ServeHTTP(full, httptest.NewRequest(http.MethodGet, "/media/show-1/episode-1/audio.mp3", nil))
	if full.Code != http.StatusOK {
		t.Fatalf("full status = %d, want 200", full.Code)
	}
	if full.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("content type = %q", full.Header().Get("Content-Type"))
	}
	if full.Header().Get("Content-Length") != "16" {
		t.Fatalf("content length = %q, want 16", full.Header().Get("Content-Length"))
	}
	if full.Body.String() != string(content) {
		t.Fatalf("body = %q", full.Body.String())
	}

	partialReq := httptest.NewRequest(http.MethodGet, "/media/show-1/episode-1/audio.mp3", nil)
	partialReq.Header.Set("Range", "bytes=4-7")
	partial := httptest.NewRecorder()
	handler.ServeHTTP(partial, partialReq)

	if partial.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d, want 206", partial.Code)
	}
	if partial.Header().Get("Content-Length") != "4" {
		t.Fatalf("range content length = %q, want 4", partial.Header().Get("Content-Length"))
	}
	if partial.Body.String() != "4567" {
		t.Fatalf("range body = %q", partial.Body.String())
	}
}

func TestHandlerRejectsUnsafeMediaPath(t *testing.T) {
	handler := media.Handler(storage.NewMemory())
	req := httptest.NewRequest(http.MethodGet, "/media/show-1/../episode/audio.mp3", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		body, _ := io.ReadAll(rr.Body)
		t.Fatalf("status = %d body = %q, want 400", rr.Code, body)
	}
}
