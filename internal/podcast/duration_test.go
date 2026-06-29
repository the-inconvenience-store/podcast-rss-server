package podcast_test

import (
	"testing"

	"github.com/samstevens/podcast-rss/internal/podcast"
)

func TestParseDurationAcceptsSecondsAndClockFormat(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int
	}{
		{input: "45", want: 45},
		{input: "01:02", want: 62},
		{input: "01:02:03", want: 3723},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := podcast.ParseDuration(tc.input)
			if err != nil {
				t.Fatalf("ParseDuration returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("seconds = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseDurationRejectsInvalidValues(t *testing.T) {
	for _, input := range []string{"", "-1", "1:2:3:4", "abc", "01:99", "01:02:99"} {
		t.Run(input, func(t *testing.T) {
			if _, err := podcast.ParseDuration(input); err == nil {
				t.Fatal("ParseDuration returned nil error")
			}
		})
	}
}
