package feed_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/samstevens/podcast-rss/internal/feed"
	"github.com/samstevens/podcast-rss/internal/podcast"
)

func TestGenerateMinimalFeedMatchesGoldenXML(t *testing.T) {
	now := mustTime(t, "2025-01-07T10:00:00Z")
	show := podcast.Show{
		ID:          "show-1",
		Title:       "Quiet Signals",
		Description: "Careful conversations about software.",
		Link:        "https://example.com",
		Language:    "en-us",
		Author:      "Sam Stevens",
		Email:       "sam@example.com",
		Category:    "Technology",
		ImageURL:    "https://cdn.example.com/quiet.png",
		Explicit:    false,
		Type:        "episodic",
		Episodes: []podcast.Episode{{
			ID:              "episode-1",
			GUID:            "11111111-1111-4111-8111-111111111111",
			Title:           "Introductions",
			Description:     "Meet the show.",
			PublicationDate: mustTime(t, "2025-01-06T10:00:00Z"),
			DurationSeconds: 61,
			AudioFileName:   "intro.mp3",
			AudioSize:       1234567,
			AudioMIME:       "audio/mpeg",
		}},
	}

	got, err := feed.Generate(show, feed.Options{
		PublicBaseURL: "https://podcasts.example.com",
		FeedPath:      "/",
		Now:           now,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	assertGolden(t, "minimal.golden.xml", got)
}

func TestGenerateFullFeedMatchesGoldenXMLAndExcludesFutureEpisodes(t *testing.T) {
	now := mustTime(t, "2025-01-07T10:00:00Z")
	show := podcast.Show{
		ID:          "workbench",
		GUID:        "22222222-2222-4222-8222-222222222222",
		Title:       "Deep Workbench",
		Description: "Build notes for independent engineers.",
		Link:        "https://example.com/workbench",
		Language:    "en-us",
		Author:      "Riley Fox",
		OwnerName:   "Riley Fox",
		Email:       "riley@example.com",
		Category:    "Technology",
		Subcategory: "Podcasting",
		ImageURL:    "https://podcasts.example.com/media/workbench/show/cover.png",
		Explicit:    true,
		Type:        "serial",
		Copyright:   "Copyright 2025 Deep Workbench",
		Locked:      true,
		Episodes: []podcast.Episode{
			{
				ID:              "future",
				GUID:            "55555555-5555-4555-8555-555555555555",
				Title:           "Not Yet",
				Description:     "This should not appear.",
				PublicationDate: mustTime(t, "2025-01-08T10:00:00Z"),
				DurationSeconds: 30,
				AudioFileName:   "future.mp3",
				AudioSize:       99,
				AudioMIME:       "audio/mpeg",
			},
			{
				ID:              "ep-1",
				GUID:            "44444444-4444-4444-8444-444444444444",
				Title:           "Trailer",
				Description:     "What this season is about.",
				PublicationDate: mustTime(t, "2025-01-05T09:30:00Z"),
				DurationSeconds: 45,
				AudioFileName:   "trailer.mp3",
				AudioSize:       10,
				AudioMIME:       "audio/mpeg",
				EpisodeType:     "trailer",
			},
			{
				ID:              "ep-2",
				GUID:            "33333333-3333-4333-8333-333333333333",
				Title:           "Shipping the Feed",
				Description:     "<p>A practical episode.</p>",
				ContentEncoded:  "<p>A practical episode.</p>",
				PublicationDate: mustTime(t, "2025-01-07T09:30:00Z"),
				DurationSeconds: 3723,
				AudioFileName:   "shipping.mp3",
				AudioSize:       7654321,
				AudioMIME:       "audio/mpeg",
				ImageURL:        "https://podcasts.example.com/media/workbench/ep-2/art.png",
				Season:          1,
				Episode:         2,
				EpisodeType:     "full",
				Explicit:        boolPtr(false),
			},
		},
	}

	got, err := feed.Generate(show, feed.Options{
		PublicBaseURL: "https://podcasts.example.com/",
		FeedPath:      "/feeds/workbench.xml",
		Now:           now,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	assertGolden(t, "full.golden.xml", got)
	if bytes.Contains(got, []byte("Not Yet")) {
		t.Fatalf("future-dated episode was included in feed:\n%s", got)
	}
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	want, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if strings.TrimSpace(string(got)) != strings.TrimSpace(string(want)) {
		t.Fatalf("feed mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}

func boolPtr(value bool) *bool {
	return &value
}
