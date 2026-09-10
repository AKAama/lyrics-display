package main

import (
	"net/http"
	"sync"
	"time"
)

type playerState string

const (
	statePlaying playerState = "playing"
	statePaused  playerState = "paused"
	stateStopped playerState = "stopped"
)

type nowPlaying struct {
	State    playerState
	Track    string
	Artist   string
	Album    string
	Position time.Duration
	Duration time.Duration
}

type lyricLine struct {
	At   time.Duration
	Text string
}

type lyricSourceKind string

const (
	lyricSourceNone    lyricSourceKind = ""
	lyricSourceMusic   lyricSourceKind = "music"
	lyricSourceApple   lyricSourceKind = "apple"
	lyricSourceNetease lyricSourceKind = "netease"
	lyricSourceLRCLIB  lyricSourceKind = "lrclib"
)

type lyricDocument struct {
	Track          string
	Artist         string
	SourceID       int64
	Lines          []lyricLine
	FetchedAt      time.Time
	DisplayName    string
	Candidates     []lyricCandidate
	SourceIndex    int
	SourceKind     lyricSourceKind
	Untimed        bool
	HadBuiltin     bool
	BuiltinLines   []lyricLine
	BuiltinUntimed bool
	BuiltinKind    lyricSourceKind
	FetchError     string
}

type lyricCandidate struct {
	ID     int64
	Name   string
	Artist string
	Score  int
	Kind   lyricSourceKind
	Synced string
}

type lyricCache struct {
	mu    sync.RWMutex
	items map[string]lyricDocument
}

func newLyricCache() *lyricCache {
	return &lyricCache{items: make(map[string]lyricDocument)}
}

func (c *lyricCache) get(key string) (lyricDocument, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	doc, ok := c.items[key]
	return doc, ok
}

func (c *lyricCache) put(key string, doc lyricDocument) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = doc
}

func (c *lyricCache) delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

type neteaseClient struct {
	http *http.Client
}

func newNetEaseClient() *neteaseClient {
	return &neteaseClient{
		http: &http.Client{Timeout: requestTimeout},
	}
}

type searchResponse struct {
	Result struct {
		Songs []struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
		} `json:"songs"`
	} `json:"result"`
}

type lyricResponse struct {
	LRC struct {
		Lyric string `json:"lyric"`
	} `json:"lrc"`
}
