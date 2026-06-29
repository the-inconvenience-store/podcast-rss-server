package podcast

import "time"

type Show struct {
	ID            string
	GUID          string
	Title         string
	Description   string
	Link          string
	Language      string
	Author        string
	OwnerName     string
	Email         string
	Category      string
	Subcategory   string
	ImageURL      string
	ImageFileName string
	Explicit      bool
	Type          string
	Copyright     string
	Locked        bool
	Episodes      []Episode
}

type Episode struct {
	ID              string
	GUID            string
	Title           string
	Description     string
	ContentEncoded  string
	PublicationDate time.Time
	DurationSeconds int
	AudioFileName   string
	AudioSize       int64
	AudioMIME       string
	ImageURL        string
	ImageFileName   string
	Season          int
	Episode         int
	EpisodeType     string
	Explicit        *bool
}
