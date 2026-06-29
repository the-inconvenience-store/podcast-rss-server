package podcast_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/samstevens/podcast-rss/internal/podcast"
)

func TestSQLiteRepositoryPersistsShowsAndEpisodes(t *testing.T) {
	repo, err := podcast.OpenSQLiteRepository(filepath.Join(t.TempDir(), "podcasts.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteRepository returned error: %v", err)
	}
	defer repo.Close()

	show := podcast.Show{
		ID:          "show-1",
		Title:       "Quiet Signals",
		Description: "Careful conversations.",
		Link:        "https://example.com",
		Language:    "en-us",
		Author:      "Sam Stevens",
		Email:       "sam@example.com",
		Category:    "Technology",
		ImageURL:    "https://cdn.example.com/cover.png",
		Type:        "episodic",
	}
	if _, err := repo.CreateShow(show); err != nil {
		t.Fatalf("CreateShow returned error: %v", err)
	}

	episode, err := repo.CreateEpisode("show-1", podcast.Episode{
		ID:              "episode-1",
		Title:           "Introductions",
		Description:     "Meet the show.",
		PublicationDate: time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC),
		DurationSeconds: 61,
		AudioFileName:   "intro.mp3",
		AudioSize:       10,
		AudioMIME:       "audio/mpeg",
	})
	if err != nil {
		t.Fatalf("CreateEpisode returned error: %v", err)
	}
	if episode.GUID == "" {
		t.Fatal("CreateEpisode did not generate a stable GUID")
	}

	got, err := repo.GetShow("show-1")
	if err != nil {
		t.Fatalf("GetShow returned error: %v", err)
	}
	if len(got.Episodes) != 1 || got.Episodes[0].GUID != episode.GUID || got.Episodes[0].AudioSize != 10 {
		t.Fatalf("persisted show = %+v", got)
	}

	got.Title = "Quiet Signals Updated"
	got.Episodes[0].Episode = 3
	if err := repo.UpdateShow(got); err != nil {
		t.Fatalf("UpdateShow returned error: %v", err)
	}
	updated, err := repo.GetShow("show-1")
	if err != nil {
		t.Fatalf("GetShow after update returned error: %v", err)
	}
	if updated.Title != "Quiet Signals Updated" || updated.Episodes[0].Episode != 3 || updated.Episodes[0].GUID != episode.GUID {
		t.Fatalf("updated show = %+v", updated)
	}

	if err := repo.DeleteEpisode("show-1", "episode-1"); err != nil {
		t.Fatalf("DeleteEpisode returned error: %v", err)
	}
	withoutEpisode, err := repo.GetShow("show-1")
	if err != nil {
		t.Fatalf("GetShow after delete episode returned error: %v", err)
	}
	if len(withoutEpisode.Episodes) != 0 {
		t.Fatalf("episodes after delete = %+v", withoutEpisode.Episodes)
	}

	if err := repo.DeleteShow("show-1"); err != nil {
		t.Fatalf("DeleteShow returned error: %v", err)
	}
	if _, err := repo.GetShow("show-1"); err == nil {
		t.Fatal("GetShow returned nil error after show delete")
	}
}
