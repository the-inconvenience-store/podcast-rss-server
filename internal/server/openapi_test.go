package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samstevens/podcast-rss/internal/podcast"
	"github.com/samstevens/podcast-rss/internal/server"
	"github.com/samstevens/podcast-rss/internal/storage"
)

func TestOpenAPISpecDocumentsEveryEndpoint(t *testing.T) {
	handler := server.New(podcast.NewMemoryRepository(), storage.NewMemory(), server.Config{
		APIKeys:       []string{"test-key"},
		PublicBaseURL: "https://podcasts.example.com",
	})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /openapi.json status = %d body=%s", rr.Code, rr.Body.String())
	}

	var spec openAPISpec
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode OpenAPI JSON: %v", err)
	}
	if spec.OpenAPI == "" {
		t.Fatal("OpenAPI version was empty")
	}
	if spec.Info.Title != "Podcast RSS Service" {
		t.Fatalf("title = %q", spec.Info.Title)
	}

	expected := map[string][]string{
		"/":                                      {"get"},
		"/feeds/{showID}.xml":                    {"get"},
		"/media/{showID}/{episodeID}/{filename}": {"get"},
		"/healthz":                               {"get"},
		"/api/shows":                             {"get", "post"},
		"/api/shows/{id}":                        {"get", "patch", "delete"},
		"/api/shows/{id}/image":                  {"post"},
		"/api/shows/{id}/episodes":               {"post"},
		"/api/shows/{id}/episodes/{eid}":         {"patch", "delete"},
		"/api/shows/{id}/episodes/{eid}/audio":   {"post"},
		"/api/shows/{id}/episodes/{eid}/image":   {"post"},
	}
	for path, methods := range expected {
		pathItem, ok := spec.Paths[path]
		if !ok {
			t.Fatalf("OpenAPI spec missing path %s", path)
		}
		for _, method := range methods {
			op, ok := pathItem[method]
			if !ok {
				t.Fatalf("OpenAPI spec missing %s %s", method, path)
			}
			if op.OperationID == "" {
				t.Fatalf("%s %s has empty operationId", method, path)
			}
			if len(op.Tags) == 0 {
				t.Fatalf("%s %s has no tags", method, path)
			}
			if pathHasAPIPrefix(path) && len(op.Security) == 0 {
				t.Fatalf("%s %s missing API key security requirement", method, path)
			}
		}
	}
	if _, ok := spec.Components.SecuritySchemes["apiKey"]; !ok {
		t.Fatal("OpenAPI spec missing apiKey security scheme")
	}
}

type openAPISpec struct {
	OpenAPI string `json:"openapi"`
	Info    struct {
		Title string `json:"title"`
	} `json:"info"`
	Paths      map[string]map[string]openAPIOperation `json:"paths"`
	Components struct {
		SecuritySchemes map[string]any `json:"securitySchemes"`
	} `json:"components"`
}

type openAPIOperation struct {
	OperationID string           `json:"operationId"`
	Tags        []string         `json:"tags"`
	Security    []map[string]any `json:"security,omitempty"`
}

func pathHasAPIPrefix(path string) bool {
	return len(path) >= len("/api/") && path[:len("/api/")] == "/api/"
}
