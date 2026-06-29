package podcast

import (
	"crypto/rand"
	"fmt"
	"sort"
	"sync"
)

type Repository interface {
	CreateShow(show Show) (Show, error)
	ListShows() ([]Show, error)
	GetShow(id string) (Show, error)
	UpdateShow(show Show) error
	DeleteShow(id string) error
	CreateEpisode(showID string, episode Episode) (Episode, error)
	UpdateEpisode(showID string, episode Episode) error
	DeleteEpisode(showID, episodeID string) error
}

type MemoryRepository struct {
	mu    sync.RWMutex
	shows map[string]Show
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{shows: make(map[string]Show)}
}

func (r *MemoryRepository) CreateShow(show Show) (Show, error) {
	if show.ID == "" {
		return Show{}, fmt.Errorf("show id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.shows[show.ID]; exists {
		return Show{}, fmt.Errorf("show already exists")
	}
	r.shows[show.ID] = cloneShow(show)
	return cloneShow(show), nil
}

func (r *MemoryRepository) ListShows() ([]Show, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	shows := make([]Show, 0, len(r.shows))
	for _, show := range r.shows {
		shows = append(shows, cloneShow(show))
	}
	sort.Slice(shows, func(i, j int) bool { return shows[i].ID < shows[j].ID })
	return shows, nil
}

func (r *MemoryRepository) GetShow(id string) (Show, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	show, ok := r.shows[id]
	if !ok {
		return Show{}, fmt.Errorf("show not found")
	}
	return cloneShow(show), nil
}

func (r *MemoryRepository) UpdateShow(show Show) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.shows[show.ID]; !ok {
		return fmt.Errorf("show not found")
	}
	r.shows[show.ID] = cloneShow(show)
	return nil
}

func (r *MemoryRepository) DeleteShow(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.shows, id)
	return nil
}

func (r *MemoryRepository) CreateEpisode(showID string, episode Episode) (Episode, error) {
	if episode.ID == "" {
		return Episode{}, fmt.Errorf("episode id is required")
	}
	if episode.GUID == "" {
		episode.GUID = randomUUID()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	show, ok := r.shows[showID]
	if !ok {
		return Episode{}, fmt.Errorf("show not found")
	}
	for _, existing := range show.Episodes {
		if existing.ID == episode.ID {
			return Episode{}, fmt.Errorf("episode already exists")
		}
	}
	show.Episodes = append(show.Episodes, episode)
	r.shows[showID] = cloneShow(show)
	return episode, nil
}

func (r *MemoryRepository) UpdateEpisode(showID string, episode Episode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	show, ok := r.shows[showID]
	if !ok {
		return fmt.Errorf("show not found")
	}
	for i := range show.Episodes {
		if show.Episodes[i].ID == episode.ID {
			show.Episodes[i] = episode
			r.shows[showID] = cloneShow(show)
			return nil
		}
	}
	return fmt.Errorf("episode not found")
}

func (r *MemoryRepository) DeleteEpisode(showID, episodeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	show, ok := r.shows[showID]
	if !ok {
		return fmt.Errorf("show not found")
	}
	episodes := show.Episodes[:0]
	for _, episode := range show.Episodes {
		if episode.ID != episodeID {
			episodes = append(episodes, episode)
		}
	}
	show.Episodes = episodes
	r.shows[showID] = cloneShow(show)
	return nil
}

func cloneShow(show Show) Show {
	show.Episodes = append([]Episode(nil), show.Episodes...)
	return show
}

func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
