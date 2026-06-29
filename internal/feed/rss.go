package feed

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/samstevens/podcast-rss/internal/podcast"
)

type Options struct {
	PublicBaseURL string
	FeedPath      string
	Now           time.Time
}

func Generate(show podcast.Show, opts Options) ([]byte, error) {
	baseURL := strings.TrimRight(opts.PublicBaseURL, "/")
	feedPath := opts.FeedPath
	if feedPath == "" {
		feedPath = "/"
	}
	feedURL := baseURL + feedPath
	ownerName := show.OwnerName
	if ownerName == "" {
		ownerName = show.Author
	}
	showGUID := show.GUID
	if showGUID == "" {
		showGUID = uuidV5URL(feedURL)
	}

	episodes := make([]podcast.Episode, 0, len(show.Episodes))
	for _, episode := range show.Episodes {
		if episode.PublicationDate.After(opts.Now) {
			continue
		}
		episodes = append(episodes, episode)
	}
	sort.SliceStable(episodes, func(i, j int) bool {
		return episodes[i].PublicationDate.After(episodes[j].PublicationDate)
	})

	items := make([]rssItem, 0, len(episodes))
	for _, episode := range episodes {
		item := rssItem{
			Title:       episode.Title,
			Enclosure:   rssEnclosure{URL: mediaURL(baseURL, show.ID, episode.ID, episode.AudioFileName), Length: episode.AudioSize, Type: episode.AudioMIME},
			GUID:        rssGUID{IsPermaLink: false, Value: episode.GUID},
			PubDate:     episode.PublicationDate.Format(time.RFC1123Z),
			Description: episode.Description,
			Duration:    formatDuration(episode.DurationSeconds),
		}
		if episode.ContentEncoded != "" {
			item.ContentEncoded = &cdataText{Value: episode.ContentEncoded}
		}
		if episode.ImageURL != "" {
			item.Image = &rssImage{HRef: episode.ImageURL}
		}
		if episode.Episode > 0 {
			item.Episode = episode.Episode
		}
		if episode.Season > 0 {
			item.Season = episode.Season
		}
		if episode.EpisodeType != "" {
			item.EpisodeType = episode.EpisodeType
		}
		if episode.Explicit != nil {
			item.Explicit = boolString(*episode.Explicit)
		}
		items = append(items, item)
	}

	channel := rssChannel{
		Title:         show.Title,
		Description:   show.Description,
		Link:          show.Link,
		Language:      show.Language,
		Author:        show.Author,
		Owner:         rssOwner{Name: ownerName, Email: show.Email},
		Image:         rssImage{HRef: show.ImageURL},
		Category:      rssCategory{Text: show.Category},
		Explicit:      boolString(show.Explicit),
		Type:          show.Type,
		AtomLink:      rssAtomLink{Rel: "self", Type: "application/rss+xml", HRef: feedURL},
		PodcastGUID:   showGUID,
		Copyright:     show.Copyright,
		LastBuildDate: opts.Now.Format(time.RFC1123Z),
		Items:         items,
	}
	if show.Subcategory != "" {
		channel.Category.Subcategory = &rssCategory{Text: show.Subcategory}
	}
	if show.Locked {
		channel.Locked = &rssLocked{Owner: show.Email, Value: "yes"}
	}

	doc := rssDocument{
		Version:   "2.0",
		ITunesNS:  "http://www.itunes.com/dtds/podcast-1.0.dtd",
		ContentNS: "http://purl.org/rss/1.0/modules/content/",
		AtomNS:    "http://www.w3.org/2005/Atom",
		PodcastNS: "https://podcastindex.org/namespace/1.0",
		Channel:   channel,
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")
	if err := encoder.Encode(doc); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

type rssDocument struct {
	XMLName   xml.Name   `xml:"rss"`
	Version   string     `xml:"version,attr"`
	ITunesNS  string     `xml:"xmlns:itunes,attr"`
	ContentNS string     `xml:"xmlns:content,attr"`
	AtomNS    string     `xml:"xmlns:atom,attr"`
	PodcastNS string     `xml:"xmlns:podcast,attr"`
	Channel   rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string      `xml:"title"`
	Description   string      `xml:"description"`
	Link          string      `xml:"link"`
	Language      string      `xml:"language"`
	Author        string      `xml:"itunes:author"`
	Owner         rssOwner    `xml:"itunes:owner"`
	Image         rssImage    `xml:"itunes:image"`
	Category      rssCategory `xml:"itunes:category"`
	Explicit      string      `xml:"itunes:explicit"`
	Type          string      `xml:"itunes:type"`
	AtomLink      rssAtomLink `xml:"atom:link"`
	PodcastGUID   string      `xml:"podcast:guid"`
	Copyright     string      `xml:"copyright,omitempty"`
	Locked        *rssLocked  `xml:"podcast:locked,omitempty"`
	LastBuildDate string      `xml:"lastBuildDate"`
	Items         []rssItem   `xml:"item"`
}

type rssOwner struct {
	Name  string `xml:"itunes:name"`
	Email string `xml:"itunes:email"`
}

type rssImage struct {
	HRef string `xml:"href,attr"`
}

type rssCategory struct {
	Text        string       `xml:"text,attr"`
	Subcategory *rssCategory `xml:"itunes:category,omitempty"`
}

type rssAtomLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
	HRef string `xml:"href,attr"`
}

type rssLocked struct {
	Owner string `xml:"owner,attr"`
	Value string `xml:",chardata"`
}

type rssItem struct {
	Title          string       `xml:"title"`
	Enclosure      rssEnclosure `xml:"enclosure"`
	GUID           rssGUID      `xml:"guid"`
	PubDate        string       `xml:"pubDate"`
	Description    string       `xml:"description"`
	ContentEncoded *cdataText   `xml:"content:encoded,omitempty"`
	Duration       string       `xml:"itunes:duration"`
	Image          *rssImage    `xml:"itunes:image,omitempty"`
	Episode        int          `xml:"itunes:episode,omitempty"`
	Season         int          `xml:"itunes:season,omitempty"`
	EpisodeType    string       `xml:"itunes:episodeType,omitempty"`
	Explicit       string       `xml:"itunes:explicit,omitempty"`
}

type rssEnclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type rssGUID struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type cdataText struct {
	Value string `xml:",cdata"`
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func mediaURL(baseURL, showID, episodeID, fileName string) string {
	return baseURL + "/media/" + path.Join(showID, episodeID, fileName)
}

func formatDuration(seconds int) string {
	if seconds >= 3600 {
		return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, seconds%3600/60, seconds%60)
	}
	if seconds >= 60 {
		return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%d", seconds)
}

func uuidV5URL(name string) string {
	namespaceURL := []byte{0x6b, 0xa7, 0xb8, 0x11, 0x9d, 0xad, 0x11, 0xd1, 0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8}
	h := sha1.New()
	h.Write(namespaceURL)
	h.Write([]byte(name))
	sum := h.Sum(nil)
	u := make([]byte, 16)
	copy(u, sum)
	u[6] = (u[6] & 0x0f) | 0x50
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(u[0:4]),
		hex.EncodeToString(u[4:6]),
		hex.EncodeToString(u[6:8]),
		hex.EncodeToString(u[8:10]),
		hex.EncodeToString(u[10:16]),
	)
}
