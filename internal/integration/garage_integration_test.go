//go:build integration

package integration_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samstevens/podcast-rss/internal/podcast"
	"github.com/samstevens/podcast-rss/internal/server"
	"github.com/samstevens/podcast-rss/internal/storage"
)

func TestGarageUploadMediaRangeAndFeedRoundTrip(t *testing.T) {
	if os.Getenv("RUN_GARAGE_INTEGRATION") != "1" {
		t.Skip("set RUN_GARAGE_INTEGRATION=1 with docker compose Garage running")
	}
	store, err := storage.NewS3Storage(storage.S3Config{
		Endpoint:        getenv("S3_ENDPOINT", "http://localhost:3900"),
		Region:          getenv("S3_REGION", "garage"),
		Bucket:          getenv("S3_BUCKET", "podcasts"),
		AccessKeyID:     getenv("S3_ACCESS_KEY_ID", "podcast-local-access-key"),
		SecretAccessKey: getenv("S3_SECRET_ACCESS_KEY", "podcast-local-secret-key-change-me"),
	})
	if err != nil {
		t.Fatalf("NewS3Storage: %v", err)
	}
	repo, err := podcast.OpenSQLiteRepository(filepath.Join(t.TempDir(), "podcasts.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteRepository: %v", err)
	}
	defer repo.Close()
	handler := server.New(repo, store, server.Config{APIKeys: []string{"test-key"}, PublicBaseURL: "https://podcasts.example.test"})

	postJSON(t, handler, "/api/shows", `{"id":"show-1","title":"Garage Show","description":"Integration","link":"https://example.com","language":"en-us","author":"Sam","email":"sam@example.com","category":"Technology","image":"https://example.com/cover.png","type":"episodic"}`)
	postJSON(t, handler, "/api/shows/show-1/episodes", `{"id":"episode-1","title":"Round trip","description":"Audio","publication_date":"2025-01-06T10:00:00Z","duration":"10"}`)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "roundtrip.mp3")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = part.Write([]byte("0123456789"))
	_ = writer.Close()
	req := authReq(http.MethodPost, "/api/shows/show-1/episodes/episode-1/audio", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", rr.Code, rr.Body.String())
	}

	rangeReq := httptest.NewRequest(http.MethodGet, "/media/show-1/episode-1/roundtrip.mp3", nil)
	rangeReq.Header.Set("Range", "bytes=3-6")
	rangeResp := httptest.NewRecorder()
	handler.ServeHTTP(rangeResp, rangeReq)
	if rangeResp.Code != http.StatusPartialContent || rangeResp.Body.String() != "3456" {
		t.Fatalf("range status=%d body=%q", rangeResp.Code, rangeResp.Body.String())
	}

	feedResp := httptest.NewRecorder()
	handler.ServeHTTP(feedResp, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(feedResp.Body.String(), `length="10" type="audio/mpeg"`) {
		t.Fatalf("feed missing enclosure metadata:\n%s", feedResp.Body.String())
	}
}

func postJSON(t *testing.T, handler http.Handler, path, body string) {
	t.Helper()
	req := authReq(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST %s status=%d body=%s", path, rr.Code, rr.Body.String())
	}
}

func authReq(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer test-key")
	return req
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
