package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var lrcPattern = regexp.MustCompile(`\[(\d{2,}):(\d{2})(?:\.(\d{1,3}))?\]([^\n\r]*)`)

func (c *neteaseClient) fetchLyrics(ctx context.Context, track, artist string) (lyricDocument, error) {
	songID, canonicalName, err := c.searchBestSong(ctx, track, artist)
	if err != nil {
		return lyricDocument{}, err
	}

	endpoint := fmt.Sprintf("https://music.163.com/api/song/lyric?id=%d&lv=1&tv=0", songID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return lyricDocument{}, err
	}

	addNetEaseHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return lyricDocument{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return lyricDocument{}, fmt.Errorf("lyric request failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload lyricResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return lyricDocument{}, err
	}

	lines := parseLRC(payload.LRC.Lyric)
	if len(lines) == 0 {
		return lyricDocument{}, fmt.Errorf("no timed lyrics found for %s - %s", track, artist)
	}

	return lyricDocument{
		Track:       track,
		Artist:      artist,
		SourceID:    songID,
		Lines:       lines,
		FetchedAt:   time.Now(),
		DisplayName: canonicalName,
	}, nil
}

func (c *neteaseClient) searchBestSong(ctx context.Context, track, artist string) (int64, string, error) {
	query := strings.TrimSpace(track + " " + artist)
	endpoint := "https://music.163.com/api/search/get?s=" + url.QueryEscape(query) + "&type=1&limit=8"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, "", err
	}

	addNetEaseHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, "", fmt.Errorf("search request failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, "", err
	}

	if len(payload.Result.Songs) == 0 {
		return 0, "", fmt.Errorf("no songs found for %s", query)
	}

	type candidate struct {
		ID    int64
		Name  string
		Score int
	}

	expectedTrack := normalizeSongName(track)
	expectedArtist := normalizeArtistName(artist)

	var matches []candidate
	for _, song := range payload.Result.Songs {
		score := similarityScore(expectedTrack, normalizeSongName(song.Name))
		artistScore := 0
		for _, item := range song.Artists {
			artistScore = max(artistScore, similarityScore(expectedArtist, normalizeArtistName(item.Name)))
		}

		matches = append(matches, candidate{
			ID:    song.ID,
			Name:  song.Name,
			Score: score*3 + artistScore*2,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	best := matches[0]
	return best.ID, best.Name, nil
}

func addNetEaseHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Referer", "https://music.163.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
}

func parseLRC(raw string) []lyricLine {
	var lines []lyricLine

	for _, line := range strings.Split(raw, "\n") {
		matches := lrcPattern.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			continue
		}

		text := strings.TrimSpace(matches[len(matches)-1][4])
		if text == "" {
			continue
		}

		for _, match := range matches {
			minutes, _ := strconv.Atoi(match[1])
			seconds, _ := strconv.Atoi(match[2])
			millis := parseFractionMillis(match[3])

			at := time.Duration(minutes)*time.Minute +
				time.Duration(seconds)*time.Second +
				time.Duration(millis)*time.Millisecond

			lines = append(lines, lyricLine{
				At:   at,
				Text: text,
			})
		}
	}

	sort.Slice(lines, func(i, j int) bool {
		return lines[i].At < lines[j].At
	})

	return lines
}

func parseFractionMillis(fragment string) int {
	switch len(fragment) {
	case 0:
		return 0
	case 1:
		value, _ := strconv.Atoi(fragment)
		return value * 100
	case 2:
		value, _ := strconv.Atoi(fragment)
		return value * 10
	default:
		value, _ := strconv.Atoi(fragment[:3])
		return value
	}
}

func currentLyric(lines []lyricLine, at time.Duration) string {
	if len(lines) == 0 {
		return ""
	}

	index := sort.Search(len(lines), func(i int) bool {
		return lines[i].At > at
	})

	if index == 0 {
		return ""
	}

	return lines[index-1].Text
}

func trimForMenuBar(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= menuBarMaxRunes {
		return string(runes)
	}

	return string(runes[:menuBarMaxRunes-1]) + "…"
}

func fallbackLine(track, artist string) string {
	track = strings.TrimSpace(track)
	artist = strings.TrimSpace(artist)
	if track == "" && artist == "" {
		return "等待播放"
	}
	if artist == "" {
		return track
	}
	if track == "" {
		return artist
	}
	return track + " - " + artist
}

func trackKey(track, artist string) string {
	return normalizeSongName(track) + "::" + normalizeArtistName(artist)
}

func normalizeSongName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"（", "(",
		"）", ")",
		"【", "[",
		"】", "]",
		"　", " ",
	)
	value = replacer.Replace(value)

	patterns := []string{
		`\([^)]*(live|version|ver\.|remaster|伴奏|纯音乐)[^)]*\)`,
		`\[[^\]]*(live|version|ver\.|remaster|伴奏|纯音乐)[^\]]*\]`,
		`\s+-\s+live.*$`,
	}

	for _, pattern := range patterns {
		value = regexp.MustCompile(pattern).ReplaceAllString(value, "")
	}

	return compactWhitespace(value)
}

func normalizeArtistName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("&", " ", ",", " ", "/", " ", "、", " ").Replace(value)
	return compactWhitespace(value)
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func similarityScore(expected, actual string) int {
	if expected == "" || actual == "" {
		return 0
	}
	if expected == actual {
		return 100
	}
	if strings.Contains(actual, expected) || strings.Contains(expected, actual) {
		return 80
	}

	expectedTokens := strings.Fields(expected)
	actualTokens := strings.Fields(actual)
	score := 0
	for _, token := range expectedTokens {
		for _, actualToken := range actualTokens {
			if token == actualToken {
				score += 20
				break
			}
		}
	}

	return min(score, 70)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
