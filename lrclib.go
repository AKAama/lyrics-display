package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type lrclibClient struct {
	http *http.Client
}

func newLRCLIBClient(httpClient *http.Client) *lrclibClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}
	return &lrclibClient{http: httpClient}
}

type lrclibSearchItem struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	SyncedLyrics string  `json:"syncedLyrics"`
	PlainLyrics  string  `json:"plainLyrics"`
}

func (c *lrclibClient) fetch(ctx context.Context, track, artist string) (lyricDocument, error) {
	candidates, err := c.searchCandidates(ctx, track, artist)
	if err != nil {
		return lyricDocument{}, err
	}

	for index := range candidates {
		doc, err := documentFromCandidate(track, artist, candidates, index)
		if err == nil {
			return doc, nil
		}
	}

	return lyricDocument{Candidates: candidates}, fmt.Errorf("no timed lyrics found on LRCLIB for %s - %s", track, artist)
}

func (c *lrclibClient) searchCandidates(ctx context.Context, track, artist string) ([]lyricCandidate, error) {
	query := url.Values{}
	query.Set("track_name", strings.TrimSpace(track))
	if strings.TrimSpace(artist) != "" {
		query.Set("artist_name", strings.TrimSpace(artist))
	}
	endpoint := "https://lrclib.net/api/search?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "lyrics-display/0.2 (https://github.com/AKAama/lyrics-display)")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("lrclib search failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var items []lrclibSearchItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no songs found on LRCLIB for %s - %s", track, artist)
	}

	expectedTrack := normalizeSongName(track)
	expectedArtist := normalizeArtistName(artist)

	var matches []lyricCandidate
	for _, item := range items {
		if item.Instrumental || strings.TrimSpace(item.SyncedLyrics) == "" {
			continue
		}
		name := firstNonEmpty(item.TrackName, item.Name)
		score := similarityScore(expectedTrack, normalizeSongName(name))
		artistScore := similarityScore(expectedArtist, normalizeArtistName(item.ArtistName))
		matches = append(matches, lyricCandidate{
			ID:     item.ID,
			Name:   name,
			Artist: item.ArtistName,
			Score:  score*3 + artistScore*2,
			Kind:   lyricSourceLRCLIB,
			Synced: item.SyncedLyrics,
		})
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no synced lyrics found on LRCLIB for %s - %s", track, artist)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})
	if len(matches) > 8 {
		matches = matches[:8]
	}
	return matches, nil
}

func (c *lrclibClient) syncedByID(ctx context.Context, id int64) (string, error) {
	endpoint := fmt.Sprintf("https://lrclib.net/api/get/%d", id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "lyrics-display/0.2 (https://github.com/AKAama/lyrics-display)")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("lrclib get failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var item lrclibSearchItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return "", err
	}
	if strings.TrimSpace(item.SyncedLyrics) == "" {
		return "", fmt.Errorf("lrclib id %d has no synced lyrics", id)
	}
	return item.SyncedLyrics, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func documentFromCandidate(track, artist string, candidates []lyricCandidate, index int) (lyricDocument, error) {
	if index < 0 || index >= len(candidates) {
		return lyricDocument{}, fmt.Errorf("lyric candidate index out of range")
	}
	candidate := candidates[index]
	lines := parseLRC(candidate.Synced)
	if len(lines) == 0 {
		return lyricDocument{}, fmt.Errorf("no timed lyrics found for %s - %s", track, artist)
	}
	kind := candidate.Kind
	if kind == "" {
		kind = lyricSourceNetease
	}
	return lyricDocument{
		Track:       track,
		Artist:      artist,
		SourceID:    candidate.ID,
		Lines:       lines,
		FetchedAt:   time.Now(),
		DisplayName: candidate.Name,
		Candidates:  candidates,
		SourceIndex: index,
		SourceKind:  kind,
	}, nil
}
