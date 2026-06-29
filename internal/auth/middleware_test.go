package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareAcceptsBearerTokenAndXAPIKey(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		value  string
	}{
		{name: "bearer", header: "Authorization", value: "Bearer secret-1"},
		{name: "x api key", header: "X-API-Key", value: "secret-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/shows", nil)
			req.Header.Set(tc.header, tc.value)
			rr := httptest.NewRecorder()
			called := false

			Middleware([]string{"secret-1"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(rr, req)

			if !called {
				t.Fatal("next handler was not called")
			}
			if rr.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
			}
		})
	}
}

func TestMiddlewareRejectsMissingAndInvalidKeys(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*http.Request)
	}{
		{name: "missing", configure: func(r *http.Request) {}},
		{name: "invalid bearer", configure: func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") }},
		{name: "wrong scheme", configure: func(r *http.Request) { r.Header.Set("Authorization", "Basic secret-1") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/shows", nil)
			tc.configure(req)
			rr := httptest.NewRecorder()

			Middleware([]string{"secret-1"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("next handler should not be called")
			})).ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
			}
			if rr.Body.String() != "unauthorized\n" {
				t.Fatalf("body = %q, want generic unauthorized message", rr.Body.String())
			}
		})
	}
}

func TestMiddlewareAcceptsAnyConfiguredKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/shows", nil)
	req.Header.Set("Authorization", "Bearer secret-2")
	rr := httptest.NewRecorder()

	Middleware([]string{"secret-1", "secret-2"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
}

func TestCheckerComparesAgainstEveryConfiguredKey(t *testing.T) {
	var calls int
	previous := constantTimeCompare
	constantTimeCompare = func(a, b []byte) int {
		calls++
		return previous(a, b)
	}
	defer func() { constantTimeCompare = previous }()

	checker := NewChecker([]string{"secret-1", "secret-2", "secret-3"})
	if !checker.Valid("secret-1") {
		t.Fatal("first key should be valid")
	}
	if calls != 3 {
		t.Fatalf("constant-time compare calls = %d, want 3", calls)
	}
}
