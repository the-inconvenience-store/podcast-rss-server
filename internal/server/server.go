package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/samstevens/podcast-rss/internal/auth"
	"github.com/samstevens/podcast-rss/internal/feed"
	"github.com/samstevens/podcast-rss/internal/media"
	"github.com/samstevens/podcast-rss/internal/podcast"
	"github.com/samstevens/podcast-rss/internal/safe"
	"github.com/samstevens/podcast-rss/internal/storage"
	"github.com/samstevens/podcast-rss/internal/validation"
)

type Config struct {
	APIKeys       []string
	PublicBaseURL string
	DefaultShowID string
	Now           func() time.Time
}

type Server struct {
	repo  podcast.Repository
	store storage.Storage
	cfg   Config
}

const apiVersion = "0.1.0"

func New(repo podcast.Repository, store storage.Storage, cfg Config) http.Handler {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	s := &Server{repo: repo, store: store, cfg: cfg}
	mux := http.NewServeMux()
	humaAPI := humago.New(mux, humaConfig())
	documentOperations(humaAPI)

	authenticated := auth.Middleware(cfg.APIKeys)
	mediaHandler := media.Handler(store)
	mux.HandleFunc("GET /{$}", s.rootFeed)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /feeds/{feedFile}", s.namedFeed)
	mux.Handle("GET /media/{showID}/{episodeID}/{filename}", mediaHandler)
	mux.Handle("GET /api/shows", authenticated(http.HandlerFunc(s.listShows)))
	mux.Handle("POST /api/shows", authenticated(http.HandlerFunc(s.createShow)))
	mux.Handle("GET /api/shows/{id}", authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.getShow(w, r, r.PathValue("id"))
	})))
	mux.Handle("PATCH /api/shows/{id}", authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.patchShow(w, r, r.PathValue("id"))
	})))
	mux.Handle("DELETE /api/shows/{id}", authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.deleteShow(w, r, r.PathValue("id"))
	})))
	mux.Handle("POST /api/shows/{id}/image", authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.uploadShowImage(w, r, r.PathValue("id"))
	})))
	mux.Handle("POST /api/shows/{id}/episodes", authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.createEpisode(w, r, r.PathValue("id"))
	})))
	mux.Handle("PATCH /api/shows/{id}/episodes/{eid}", authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.patchEpisode(w, r, r.PathValue("id"), r.PathValue("eid"))
	})))
	mux.Handle("DELETE /api/shows/{id}/episodes/{eid}", authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.deleteEpisode(w, r, r.PathValue("id"), r.PathValue("eid"))
	})))
	mux.Handle("POST /api/shows/{id}/episodes/{eid}/audio", authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.uploadEpisodeAudio(w, r, r.PathValue("id"), r.PathValue("eid"))
	})))
	mux.Handle("POST /api/shows/{id}/episodes/{eid}/image", authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.uploadEpisodeImage(w, r, r.PathValue("id"), r.PathValue("eid"))
	})))
	return mux
}

func humaConfig() huma.Config {
	cfg := huma.DefaultConfig("Podcast RSS Service", apiVersion)
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"apiKey": {
			Type:        "http",
			Scheme:      "bearer",
			Description: "API key supplied as `Authorization: Bearer <key>`. `X-API-Key` is also accepted by the service.",
		},
	}
	return cfg
}

func documentOperations(api huma.API) {
	for _, op := range []huma.Operation{
		publicOperation(http.MethodGet, "/", "get-primary-feed", "Feeds", "Get primary podcast RSS feed", "application/rss+xml"),
		publicOperation(http.MethodGet, "/feeds/{showID}.xml", "get-show-feed", "Feeds", "Get podcast RSS feed by show ID", "application/rss+xml", pathParam("showID")),
		publicOperation(http.MethodGet, "/media/{showID}/{episodeID}/{filename}", "get-media", "Media", "Get proxied podcast media with HTTP range support", "application/octet-stream", pathParam("showID"), pathParam("episodeID"), pathParam("filename")),
		{
			Method:      http.MethodGet,
			Path:        "/healthz",
			OperationID: "get-healthz",
			Tags:        []string{"Health"},
			Summary:     "Health check",
			Responses: map[string]*huma.Response{
				"204": {Description: "Service is healthy"},
			},
		},
		apiOperation(http.MethodGet, "/api/shows", "list-shows", "Shows", "List shows", nil),
		apiOperation(http.MethodPost, "/api/shows", "create-show", "Shows", "Create show", jsonRequest("Show metadata")),
		apiOperation(http.MethodGet, "/api/shows/{id}", "get-show", "Shows", "Get show", nil, pathParam("id")),
		apiOperation(http.MethodPatch, "/api/shows/{id}", "patch-show", "Shows", "Update show", jsonRequest("Partial show metadata"), pathParam("id")),
		apiOperation(http.MethodDelete, "/api/shows/{id}", "delete-show", "Shows", "Delete show and stored objects", nil, pathParam("id")),
		apiOperation(http.MethodPost, "/api/shows/{id}/image", "upload-show-image", "Uploads", "Upload show cover artwork", multipartRequest("JPEG or PNG cover artwork"), pathParam("id")),
		apiOperation(http.MethodPost, "/api/shows/{id}/episodes", "create-episode", "Episodes", "Create episode", jsonRequest("Episode metadata"), pathParam("id")),
		apiOperation(http.MethodPatch, "/api/shows/{id}/episodes/{eid}", "patch-episode", "Episodes", "Update episode", jsonRequest("Partial episode metadata"), pathParam("id"), pathParam("eid")),
		apiOperation(http.MethodDelete, "/api/shows/{id}/episodes/{eid}", "delete-episode", "Episodes", "Delete episode and stored objects", nil, pathParam("id"), pathParam("eid")),
		apiOperation(http.MethodPost, "/api/shows/{id}/episodes/{eid}/audio", "upload-episode-audio", "Uploads", "Upload episode audio", multipartRequest("Episode audio file"), pathParam("id"), pathParam("eid")),
		apiOperation(http.MethodPost, "/api/shows/{id}/episodes/{eid}/image", "upload-episode-image", "Uploads", "Upload episode artwork", multipartRequest("JPEG or PNG episode artwork"), pathParam("id"), pathParam("eid")),
	} {
		current := op
		api.OpenAPI().AddOperation(&current)
	}
}

func publicOperation(method, path, id, tag, summary, contentType string, params ...*huma.Param) huma.Operation {
	return huma.Operation{
		Method:      method,
		Path:        path,
		OperationID: id,
		Tags:        []string{tag},
		Summary:     summary,
		Parameters:  params,
		Responses: map[string]*huma.Response{
			"200": response("OK", contentType),
		},
	}
}

func apiOperation(method, path, id, tag, summary string, body *huma.RequestBody, params ...*huma.Param) huma.Operation {
	status := "200"
	if method == http.MethodPost {
		status = "201"
	}
	if method == http.MethodDelete {
		status = "204"
	}
	op := huma.Operation{
		Method:      method,
		Path:        path,
		OperationID: id,
		Tags:        []string{tag},
		Summary:     summary,
		Parameters:  params,
		RequestBody: body,
		Security:    []map[string][]string{{"apiKey": {}}},
		Responses: map[string]*huma.Response{
			status: response("Success", "application/json"),
			"401":  {Description: "Missing or invalid API key"},
		},
	}
	if method == http.MethodDelete {
		op.Responses = map[string]*huma.Response{
			status: {Description: "Deleted"},
			"401":  {Description: "Missing or invalid API key"},
		}
	}
	return op
}

func response(description, contentType string) *huma.Response {
	return &huma.Response{
		Description: description,
		Content: map[string]*huma.MediaType{
			contentType: {Schema: &huma.Schema{Type: "string"}},
		},
	}
}

func jsonRequest(description string) *huma.RequestBody {
	return &huma.RequestBody{
		Description: description,
		Required:    true,
		Content: map[string]*huma.MediaType{
			"application/json": {Schema: &huma.Schema{Type: "object"}},
		},
	}
}

func multipartRequest(description string) *huma.RequestBody {
	return &huma.RequestBody{
		Description: description,
		Required:    true,
		Content: map[string]*huma.MediaType{
			"multipart/form-data": {
				Schema: &huma.Schema{
					Type: "object",
					Properties: map[string]*huma.Schema{
						"file": {Type: "string", Format: "binary"},
					},
					Required: []string{"file"},
				},
			},
		},
	}
}

func pathParam(name string) *huma.Param {
	return &huma.Param{
		Name:        name,
		In:          "path",
		Required:    true,
		Description: name + " path segment",
		Schema:      &huma.Schema{Type: "string"},
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) rootFeed(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	show, err := s.primaryShow()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.writeFeed(w, show, "/")
}

func (s *Server) namedFeed(w http.ResponseWriter, r *http.Request) {
	feedFile := strings.TrimPrefix(r.URL.Path, "/feeds/")
	if !strings.HasSuffix(feedFile, ".xml") {
		http.NotFound(w, r)
		return
	}
	showID := strings.TrimSuffix(feedFile, ".xml")
	show, err := s.repo.GetShow(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.writeFeed(w, show, "/feeds/"+show.ID+".xml")
}

func (s *Server) writeFeed(w http.ResponseWriter, show podcast.Show, feedPath string) {
	xmlBytes, err := feed.Generate(show, feed.Options{
		PublicBaseURL: s.cfg.PublicBaseURL,
		FeedPath:      feedPath,
		Now:           s.cfg.Now(),
	})
	if err != nil {
		http.Error(w, "could not generate feed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(xmlBytes)
}

func (s *Server) primaryShow() (podcast.Show, error) {
	if s.cfg.DefaultShowID != "" {
		return s.repo.GetShow(s.cfg.DefaultShowID)
	}
	shows, err := s.repo.ListShows()
	if err != nil {
		return podcast.Show{}, err
	}
	if len(shows) != 1 {
		return podcast.Show{}, fmt.Errorf("primary show is ambiguous")
	}
	return shows[0], nil
}

type showRequest struct {
	ID          string `json:"id"`
	GUID        string `json:"guid"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Link        string `json:"link"`
	Language    string `json:"language"`
	Author      string `json:"author"`
	OwnerName   string `json:"owner_name"`
	Email       string `json:"email"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Image       string `json:"image"`
	Explicit    *bool  `json:"explicit"`
	Type        string `json:"type"`
	Copyright   string `json:"copyright"`
	Locked      *bool  `json:"locked"`
}

func (s *Server) createShow(w http.ResponseWriter, r *http.Request) {
	var req showRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	show := showFromRequest(req, podcast.Show{})
	created, err := s.repo.CreateShow(show)
	if err != nil {
		http.Error(w, "could not create show", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) listShows(w http.ResponseWriter, r *http.Request) {
	shows, err := s.repo.ListShows()
	if err != nil {
		http.Error(w, "could not list shows", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, shows)
}

func (s *Server) getShow(w http.ResponseWriter, r *http.Request, showID string) {
	show, err := s.repo.GetShow(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, show)
}

func (s *Server) patchShow(w http.ResponseWriter, r *http.Request, showID string) {
	show, err := s.repo.GetShow(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var req showRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	show = showFromRequest(req, show)
	if err := s.repo.UpdateShow(show); err != nil {
		http.Error(w, "could not update show", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, show)
}

func showFromRequest(req showRequest, show podcast.Show) podcast.Show {
	if req.ID != "" {
		show.ID = req.ID
	}
	if req.GUID != "" {
		show.GUID = req.GUID
	}
	if req.Title != "" {
		show.Title = req.Title
	}
	if req.Description != "" {
		show.Description = req.Description
	}
	if req.Link != "" {
		show.Link = req.Link
	}
	if req.Language != "" {
		show.Language = req.Language
	}
	if req.Author != "" {
		show.Author = req.Author
	}
	if req.OwnerName != "" {
		show.OwnerName = req.OwnerName
	}
	if req.Email != "" {
		show.Email = req.Email
	}
	if req.Category != "" {
		show.Category = req.Category
	}
	if req.Subcategory != "" {
		show.Subcategory = req.Subcategory
	}
	if req.Image != "" {
		show.ImageURL = req.Image
	}
	if req.Explicit != nil {
		show.Explicit = *req.Explicit
	}
	if req.Type != "" {
		show.Type = req.Type
	}
	if req.Copyright != "" {
		show.Copyright = req.Copyright
	}
	if req.Locked != nil {
		show.Locked = *req.Locked
	}
	return show
}

type episodeRequest struct {
	ID              string `json:"id"`
	GUID            string `json:"guid"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	ContentEncoded  string `json:"content_encoded"`
	PublicationDate string `json:"publication_date"`
	Duration        string `json:"duration"`
	EpisodeType     string `json:"episode_type"`
	Season          *int   `json:"season"`
	Episode         *int   `json:"episode"`
	Explicit        *bool  `json:"explicit"`
}

func (s *Server) createEpisode(w http.ResponseWriter, r *http.Request, showID string) {
	var req episodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	episode, err := episodeFromRequest(req, podcast.Episode{})
	if err != nil {
		http.Error(w, "invalid episode", http.StatusBadRequest)
		return
	}
	created, err := s.repo.CreateEpisode(showID, episode)
	if err != nil {
		http.Error(w, "could not create episode", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) patchEpisode(w http.ResponseWriter, r *http.Request, showID, episodeID string) {
	show, err := s.repo.GetShow(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	episode, ok := findEpisode(show, episodeID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var req episodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	episode, err = episodeFromRequest(req, episode)
	if err != nil {
		http.Error(w, "invalid episode", http.StatusBadRequest)
		return
	}
	if err := s.repo.UpdateEpisode(showID, episode); err != nil {
		http.Error(w, "could not update episode", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, episode)
}

func episodeFromRequest(req episodeRequest, episode podcast.Episode) (podcast.Episode, error) {
	if req.ID != "" {
		episode.ID = req.ID
	}
	if req.GUID != "" {
		episode.GUID = req.GUID
	}
	if req.Title != "" {
		episode.Title = req.Title
	}
	if req.Description != "" {
		episode.Description = req.Description
	}
	if req.ContentEncoded != "" {
		episode.ContentEncoded = req.ContentEncoded
	}
	if req.PublicationDate != "" {
		parsed, err := time.Parse(time.RFC3339, req.PublicationDate)
		if err != nil {
			return podcast.Episode{}, err
		}
		episode.PublicationDate = parsed
	}
	if req.Duration != "" {
		seconds, err := podcast.ParseDuration(req.Duration)
		if err != nil {
			return podcast.Episode{}, err
		}
		episode.DurationSeconds = seconds
	}
	if req.EpisodeType != "" {
		episode.EpisodeType = req.EpisodeType
	}
	if req.Season != nil {
		episode.Season = *req.Season
	}
	if req.Episode != nil {
		episode.Episode = *req.Episode
	}
	if req.Explicit != nil {
		episode.Explicit = req.Explicit
	}
	return episode, nil
}

func (s *Server) uploadEpisodeAudio(w http.ResponseWriter, r *http.Request, showID, episodeID string) {
	show, err := s.repo.GetShow(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	episode, ok := findEpisode(show, episodeID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	file, filename, contentType, err := multipartFile(r)
	if err != nil {
		http.Error(w, "invalid upload", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if contentType == "" || contentType == "application/octet-stream" {
		if detected := mime.TypeByExtension(filepath.Ext(filename)); detected != "" {
			contentType = detected
		}
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	key, err := safe.MediaObjectKey(showID, episodeID, filename)
	if err != nil {
		http.Error(w, "unsafe filename", http.StatusBadRequest)
		return
	}
	counter := &countingReader{r: file}
	if err := s.store.Put(key, counter, storage.ObjectInfo{ContentType: contentType}); err != nil {
		http.Error(w, "could not store audio", http.StatusInternalServerError)
		return
	}
	episode.AudioFileName = filename
	episode.AudioMIME = contentType
	episode.AudioSize = counter.n
	if err := s.repo.UpdateEpisode(showID, episode); err != nil {
		http.Error(w, "could not update episode", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, episode)
}

func (s *Server) uploadShowImage(w http.ResponseWriter, r *http.Request, showID string) {
	show, err := s.repo.GetShow(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	filename, info, err := s.storeArtwork(r, showID, "show")
	if err != nil {
		http.Error(w, "invalid image upload", http.StatusBadRequest)
		return
	}
	show.ImageFileName = filename
	show.ImageURL = s.mediaURL(showID, "show", filename)
	if err := s.repo.UpdateShow(show); err != nil {
		http.Error(w, "could not update show", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) uploadEpisodeImage(w http.ResponseWriter, r *http.Request, showID, episodeID string) {
	show, err := s.repo.GetShow(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	episode, ok := findEpisode(show, episodeID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	filename, info, err := s.storeArtwork(r, showID, episodeID)
	if err != nil {
		http.Error(w, "invalid image upload", http.StatusBadRequest)
		return
	}
	episode.ImageFileName = filename
	episode.ImageURL = s.mediaURL(showID, episodeID, filename)
	if err := s.repo.UpdateEpisode(showID, episode); err != nil {
		http.Error(w, "could not update episode", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) storeArtwork(r *http.Request, showID, episodeID string) (string, validation.ImageInfo, error) {
	file, filename, declaredContentType, err := multipartFile(r)
	if err != nil {
		return "", validation.ImageInfo{}, err
	}
	defer file.Close()
	filename, err = safe.PathPart(filename)
	if err != nil {
		return "", validation.ImageInfo{}, err
	}
	data, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		return "", validation.ImageInfo{}, err
	}
	info, err := validation.ValidateArtwork(bytes.NewReader(data), declaredContentType)
	if err != nil {
		return "", validation.ImageInfo{}, err
	}
	key, err := safe.MediaObjectKey(showID, episodeID, filename)
	if err != nil {
		return "", validation.ImageInfo{}, err
	}
	if err := s.store.Put(key, bytes.NewReader(data), storage.ObjectInfo{
		Size:        int64(len(data)),
		ContentType: info.ContentType,
	}); err != nil {
		return "", validation.ImageInfo{}, err
	}
	return filename, info, nil
}

func (s *Server) mediaURL(showID, episodeID, filename string) string {
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/media/" + showID + "/" + episodeID + "/" + filename
}

func multipartFile(r *http.Request) (io.ReadCloser, string, string, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, "", "", err
	}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return nil, "", "", fmt.Errorf("file part not found")
		}
		if err != nil {
			return nil, "", "", err
		}
		if part.FormName() == "file" && part.FileName() != "" {
			return part, part.FileName(), part.Header.Get("Content-Type"), nil
		}
		_ = part.Close()
	}
}

func (s *Server) deleteEpisode(w http.ResponseWriter, r *http.Request, showID, episodeID string) {
	show, err := s.repo.GetShow(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	episode, ok := findEpisode(show, episodeID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.deleteEpisodeObjects(showID, episode)
	if err := s.repo.DeleteEpisode(showID, episodeID); err != nil {
		http.Error(w, "could not delete episode", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteShow(w http.ResponseWriter, r *http.Request, showID string) {
	show, err := s.repo.GetShow(showID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if show.ImageFileName != "" {
		if key, err := safe.MediaObjectKey(showID, "show", show.ImageFileName); err == nil {
			_ = s.store.Delete(key)
		}
	}
	for _, episode := range show.Episodes {
		s.deleteEpisodeObjects(showID, episode)
	}
	if err := s.repo.DeleteShow(showID); err != nil {
		http.Error(w, "could not delete show", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteEpisodeObjects(showID string, episode podcast.Episode) {
	if episode.AudioFileName != "" {
		if key, err := safe.MediaObjectKey(showID, episode.ID, episode.AudioFileName); err == nil {
			_ = s.store.Delete(key)
		}
	}
	if episode.ImageFileName != "" {
		if key, err := safe.MediaObjectKey(showID, episode.ID, episode.ImageFileName); err == nil {
			_ = s.store.Delete(key)
		}
	}
}

func findEpisode(show podcast.Show, episodeID string) (podcast.Episode, bool) {
	for _, episode := range show.Episodes {
		if episode.ID == episodeID {
			return episode, true
		}
	}
	return podcast.Episode{}, false
}

type countingReader struct {
	r io.Reader
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
