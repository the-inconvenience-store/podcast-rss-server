package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samstevens/podcast-rss/internal/podcast"
	"github.com/samstevens/podcast-rss/internal/server"
	"github.com/samstevens/podcast-rss/internal/storage"
)

func TestPublicRoutesAreOpenAndAPIRoutesRequireAKey(t *testing.T) {
	repo := podcast.NewMemoryRepository()
	store := storage.NewMemory()
	handler := server.New(repo, store, server.Config{
		APIKeys:       []string{"test-key"},
		PublicBaseURL: "https://podcasts.example.com",
	})

	for _, path := range []string{"/healthz", "/"} {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code == http.StatusUnauthorized {
			t.Fatalf("%s unexpectedly required auth", path)
		}
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/shows", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/shows without key = %d, want 401", rr.Code)
	}
}

func TestShowAndEpisodeCRUDThroughAPI(t *testing.T) {
	repo := podcast.NewMemoryRepository()
	handler := server.New(repo, storage.NewMemory(), server.Config{
		APIKeys:       []string{"test-key"},
		PublicBaseURL: "https://podcasts.example.com",
	})

	showBody := `{
		"id":"show-1",
		"title":"Quiet Signals",
		"description":"Careful conversations.",
		"link":"https://example.com",
		"language":"en-us",
		"author":"Sam Stevens",
		"email":"sam@example.com",
		"category":"Technology",
		"image":"https://cdn.example.com/cover.png",
		"explicit":false,
		"type":"episodic"
	}`
	createShow := authReq(http.MethodPost, "/api/shows", strings.NewReader(showBody))
	createShow.Header.Set("Content-Type", "application/json")
	showResp := httptest.NewRecorder()
	handler.ServeHTTP(showResp, createShow)
	if showResp.Code != http.StatusCreated {
		t.Fatalf("create show status = %d body=%s", showResp.Code, showResp.Body.String())
	}

	episodeBody := `{
		"id":"episode-1",
		"title":"Introductions",
		"description":"Meet the show.",
		"publication_date":"2025-01-06T10:00:00Z",
		"duration":"01:01",
		"episode_type":"full"
	}`
	createEpisode := authReq(http.MethodPost, "/api/shows/show-1/episodes", strings.NewReader(episodeBody))
	createEpisode.Header.Set("Content-Type", "application/json")
	episodeResp := httptest.NewRecorder()
	handler.ServeHTTP(episodeResp, createEpisode)
	if episodeResp.Code != http.StatusCreated {
		t.Fatalf("create episode status = %d body=%s", episodeResp.Code, episodeResp.Body.String())
	}

	patch := authReq(http.MethodPatch, "/api/shows/show-1/episodes/episode-1", strings.NewReader(`{"episode":7}`))
	patch.Header.Set("Content-Type", "application/json")
	patchResp := httptest.NewRecorder()
	handler.ServeHTTP(patchResp, patch)
	if patchResp.Code != http.StatusOK {
		t.Fatalf("patch episode status = %d body=%s", patchResp.Code, patchResp.Body.String())
	}

	got, err := repo.GetShow("show-1")
	if err != nil {
		t.Fatalf("GetShow returned error: %v", err)
	}
	if len(got.Episodes) != 1 || got.Episodes[0].Episode != 7 || got.Episodes[0].DurationSeconds != 61 {
		t.Fatalf("stored episode = %+v", got.Episodes)
	}

	del := authReq(http.MethodDelete, "/api/shows/show-1/episodes/episode-1", nil)
	delResp := httptest.NewRecorder()
	handler.ServeHTTP(delResp, del)
	if delResp.Code != http.StatusNoContent {
		t.Fatalf("delete episode status = %d", delResp.Code)
	}
}

func TestAudioUploadUpdatesMetadataMediaEndpointAndFeed(t *testing.T) {
	repo := podcast.NewMemoryRepository()
	store := storage.NewMemory()
	handler := server.New(repo, store, server.Config{
		APIKeys:       []string{"test-key"},
		PublicBaseURL: "https://podcasts.example.com",
		Now:           func() time.Time { return mustTime(t, "2025-01-07T10:00:00Z") },
	})
	mustCreateShowAndEpisode(t, handler)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "intro.mp3")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("0123456789")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	upload := authReq(http.MethodPost, "/api/shows/show-1/episodes/episode-1/audio", &body)
	upload.Header.Set("Content-Type", writer.FormDataContentType())
	uploadResp := httptest.NewRecorder()
	handler.ServeHTTP(uploadResp, upload)
	if uploadResp.Code != http.StatusOK {
		t.Fatalf("upload status = %d body=%s", uploadResp.Code, uploadResp.Body.String())
	}

	rangeReq := httptest.NewRequest(http.MethodGet, "/media/show-1/episode-1/intro.mp3", nil)
	rangeReq.Header.Set("Range", "bytes=2-5")
	rangeResp := httptest.NewRecorder()
	handler.ServeHTTP(rangeResp, rangeReq)
	if rangeResp.Code != http.StatusPartialContent || rangeResp.Body.String() != "2345" {
		t.Fatalf("range status=%d body=%q", rangeResp.Code, rangeResp.Body.String())
	}

	feedResp := httptest.NewRecorder()
	handler.ServeHTTP(feedResp, httptest.NewRequest(http.MethodGet, "/", nil))
	if feedResp.Code != http.StatusOK {
		t.Fatalf("feed status = %d body=%s", feedResp.Code, feedResp.Body.String())
	}
	if !strings.Contains(feedResp.Body.String(), `url="https://podcasts.example.com/media/show-1/episode-1/intro.mp3" length="10" type="audio/mpeg"`) {
		t.Fatalf("feed did not contain byte-accurate enclosure:\n%s", feedResp.Body.String())
	}
}

func TestDeletingShowRemovesStoredObjects(t *testing.T) {
	repo := podcast.NewMemoryRepository()
	store := storage.NewMemory()
	handler := server.New(repo, store, server.Config{
		APIKeys:       []string{"test-key"},
		PublicBaseURL: "https://podcasts.example.com",
	})
	mustCreateShowAndEpisode(t, handler)
	if err := store.Put("media/show-1/episode-1/intro.mp3", strings.NewReader("audio"), storage.ObjectInfo{Size: 5, ContentType: "audio/mpeg"}); err != nil {
		t.Fatalf("Put audio: %v", err)
	}
	if err := store.Put("media/show-1/show/cover.png", strings.NewReader("image"), storage.ObjectInfo{Size: 5, ContentType: "image/png"}); err != nil {
		t.Fatalf("Put image: %v", err)
	}
	show, _ := repo.GetShow("show-1")
	show.ImageFileName = "cover.png"
	show.Episodes[0].AudioFileName = "intro.mp3"
	if err := repo.UpdateShow(show); err != nil {
		t.Fatalf("UpdateShow: %v", err)
	}

	del := authReq(http.MethodDelete, "/api/shows/show-1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, del)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete show status = %d", rr.Code)
	}
	if _, err := store.Stat("media/show-1/episode-1/intro.mp3"); err == nil {
		t.Fatal("audio object still exists")
	}
	if _, err := store.Stat("media/show-1/show/cover.png"); err == nil {
		t.Fatal("show image object still exists")
	}
}

func mustCreateShowAndEpisode(t *testing.T, handler http.Handler) {
	t.Helper()
	show := authReq(http.MethodPost, "/api/shows", strings.NewReader(`{
		"id":"show-1",
		"title":"Quiet Signals",
		"description":"Careful conversations.",
		"link":"https://example.com",
		"language":"en-us",
		"author":"Sam Stevens",
		"email":"sam@example.com",
		"category":"Technology",
		"image":"https://cdn.example.com/cover.png",
		"explicit":false,
		"type":"episodic"
	}`))
	show.Header.Set("Content-Type", "application/json")
	showResp := httptest.NewRecorder()
	handler.ServeHTTP(showResp, show)
	if showResp.Code != http.StatusCreated {
		t.Fatalf("create show status=%d body=%s", showResp.Code, showResp.Body.String())
	}

	episode := authReq(http.MethodPost, "/api/shows/show-1/episodes", strings.NewReader(`{
		"id":"episode-1",
		"title":"Introductions",
		"description":"Meet the show.",
		"publication_date":"2025-01-06T10:00:00Z",
		"duration":"61"
	}`))
	episode.Header.Set("Content-Type", "application/json")
	episodeResp := httptest.NewRecorder()
	handler.ServeHTTP(episodeResp, episode)
	if episodeResp.Code != http.StatusCreated {
		t.Fatalf("create episode status=%d body=%s", episodeResp.Code, episodeResp.Body.String())
	}
}

func authReq(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer test-key")
	return req
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}

func decodeJSON(t *testing.T, body *bytes.Buffer, dst any) {
	t.Helper()
	if err := json.Unmarshal(body.Bytes(), dst); err != nil {
		t.Fatalf("decode JSON %q: %v", body.String(), err)
	}
}
