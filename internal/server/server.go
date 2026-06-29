package server

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/samstevens/podcast-rss/internal/auth"
	"github.com/samstevens/podcast-rss/internal/feed"
	"github.com/samstevens/podcast-rss/internal/media"
	"github.com/samstevens/podcast-rss/internal/podcast"
	"github.com/samstevens/podcast-rss/internal/safe"
	"github.com/samstevens/podcast-rss/internal/storage"
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

func New(repo podcast.Repository, store storage.Storage, cfg Config) http.Handler {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	s := &Server{repo: repo, store: store, cfg: cfg}
	api := auth.Middleware(cfg.APIKeys)(http.HandlerFunc(s.api))
	mediaHandler := media.Handler(store)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/"):
			api.ServeHTTP(w, r)
		case r.URL.Path == "/healthz" && r.Method == http.MethodGet:
			s.health(w, r)
		case r.URL.Path == "/" && r.Method == http.MethodGet:
			s.rootFeed(w, r)
		case strings.HasPrefix(r.URL.Path, "/feeds/") && r.Method == http.MethodGet:
			s.namedFeed(w, r)
		case strings.HasPrefix(r.URL.Path, "/media/") && r.Method == http.MethodGet:
			mediaHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
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

func (s *Server) api(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")
	switch {
	case path == "/shows" && r.Method == http.MethodPost:
		s.createShow(w, r)
	case path == "/shows" && r.Method == http.MethodGet:
		s.listShows(w, r)
	case strings.HasPrefix(path, "/shows/"):
		s.showRoute(w, r, strings.TrimPrefix(path, "/shows/"))
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) showRoute(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.Split(rest, "/")
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.getShow(w, r, parts[0])
		case http.MethodPatch:
			s.patchShow(w, r, parts[0])
		case http.MethodDelete:
			s.deleteShow(w, r, parts[0])
		default:
			http.NotFound(w, r)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "image" && r.Method == http.MethodPost {
		s.uploadShowImage(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "episodes" && r.Method == http.MethodPost {
		s.createEpisode(w, r, parts[0])
		return
	}
	if len(parts) == 3 && parts[1] == "episodes" {
		switch r.Method {
		case http.MethodPatch:
			s.patchEpisode(w, r, parts[0], parts[2])
		case http.MethodDelete:
			s.deleteEpisode(w, r, parts[0], parts[2])
		default:
			http.NotFound(w, r)
		}
		return
	}
	if len(parts) == 4 && parts[1] == "episodes" && parts[3] == "audio" && r.Method == http.MethodPost {
		s.uploadEpisodeAudio(w, r, parts[0], parts[2])
		return
	}
	if len(parts) == 4 && parts[1] == "episodes" && parts[3] == "image" && r.Method == http.MethodPost {
		s.uploadEpisodeImage(w, r, parts[0], parts[2])
		return
	}
	http.NotFound(w, r)
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
	http.Error(w, "image upload not implemented", http.StatusNotImplemented)
}

func (s *Server) uploadEpisodeImage(w http.ResponseWriter, r *http.Request, showID, episodeID string) {
	http.Error(w, "image upload not implemented", http.StatusNotImplemented)
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

func atoi(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

var _ = atoi
