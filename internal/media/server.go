package media

import (
	"net/http"
	"strings"

	"github.com/samstevens/podcast-rss/internal/safe"
	"github.com/samstevens/podcast-rss/internal/storage"
)

func Handler(store storage.Storage) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		key, err := keyFromPath(r.URL.Path)
		if err != nil {
			http.Error(w, "bad media path", http.StatusBadRequest)
			return
		}
		info, err := store.Stat(key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		body, err := store.Get(key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer body.Close()

		w.Header().Set("Content-Type", info.ContentType)
		http.ServeContent(w, r, key, info.ModTime, body)
	})
}

func keyFromPath(urlPath string) (string, error) {
	trimmed := strings.TrimPrefix(urlPath, "/media/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 || strings.HasPrefix(trimmed, "/") || trimmed == urlPath {
		return "", errBadPath{}
	}
	return safe.MediaObjectKey(parts[0], parts[1], parts[2])
}

type errBadPath struct{}

func (errBadPath) Error() string {
	return "bad path"
}
