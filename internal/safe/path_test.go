package safe_test

import (
	"testing"

	"github.com/samstevens/podcast-rss/internal/safe"
)

func TestPathPartAllowsSimpleIdentifiersAndFilenames(t *testing.T) {
	for _, input := range []string{"show-1", "Episode_02", "cover.png", "audio.final.mp3"} {
		got, err := safe.PathPart(input)
		if err != nil {
			t.Fatalf("PathPart(%q) returned error: %v", input, err)
		}
		if got != input {
			t.Fatalf("PathPart(%q) = %q", input, got)
		}
	}
}

func TestPathPartRejectsTraversalAndUnsafeValues(t *testing.T) {
	for _, input := range []string{"", ".", "..", "../secret", "a/b", `a\b`, " name", "name ", "semi;colon", "emoji-☃"} {
		t.Run(input, func(t *testing.T) {
			if _, err := safe.PathPart(input); err == nil {
				t.Fatal("PathPart returned nil error")
			}
		})
	}
}

func TestMediaObjectKeyUsesSanitisedParts(t *testing.T) {
	got, err := safe.MediaObjectKey("show-1", "episode-1", "intro.mp3")
	if err != nil {
		t.Fatalf("MediaObjectKey returned error: %v", err)
	}
	if got != "media/show-1/episode-1/intro.mp3" {
		t.Fatalf("key = %q", got)
	}

	if _, err := safe.MediaObjectKey("show-1", "../episode", "intro.mp3"); err == nil {
		t.Fatal("MediaObjectKey accepted traversal")
	}
}
