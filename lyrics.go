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
	candidates, err := c.searchCandidates(ctx, track, artist)
	if err != nil {
		return lyricDocument{}, err
	}

	for index := range candidates {
		doc, err := c.fetchLyricsForCandidate(ctx, track, artist, candidates, index)
		if err == nil {
			return doc, nil
		}
	}

	return lyricDocument{Candidates: candidates}, fmt.Errorf("no timed lyrics found for %s - %s", track, artist)
}

func (c *neteaseClient) fetchNextLyrics(ctx context.Context, track, artist string, current lyricDocument) (lyricDocument, error) {
	if len(current.Candidates) == 0 {
		return lyricDocument{}, fmt.Errorf("no candidates available")
	}

	for offset := 1; offset < len(current.Candidates)+1; offset++ {
		index := (current.SourceIndex + offset) % len(current.Candidates)
		doc, err := c.fetchLyricsForCandidate(ctx, track, artist, current.Candidates, index)
		if err == nil {
			return doc, nil
		}
	}

	return current, fmt.Errorf("no alternative lyric sources worked")
}

func (c *neteaseClient) fetchLyricsForCandidate(ctx context.Context, track, artist string, candidates []lyricCandidate, index int) (lyricDocument, error) {
	candidate := candidates[index]
	endpoint := fmt.Sprintf("https://music.163.com/api/song/lyric?id=%d&lv=1&tv=0", candidate.ID)
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
		SourceID:    candidate.ID,
		Lines:       lines,
		FetchedAt:   time.Now(),
		DisplayName: candidate.Name,
		Candidates:  candidates,
		SourceIndex: index,
		SourceKind:  lyricSourceNetease,
	}, nil
}

func (c *neteaseClient) searchCandidates(ctx context.Context, track, artist string) ([]lyricCandidate, error) {
	query := strings.TrimSpace(track + " " + artist)
	endpoint := "https://music.163.com/api/search/get?s=" + url.QueryEscape(query) + "&type=1&limit=8"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	addNetEaseHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("search request failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	if len(payload.Result.Songs) == 0 {
		return nil, fmt.Errorf("no songs found for %s", query)
	}

	expectedTrack := normalizeSongName(track)
	expectedArtist := normalizeArtistName(artist)

	var matches []lyricCandidate
	for _, song := range payload.Result.Songs {
		score := similarityScore(expectedTrack, normalizeSongName(song.Name))
		artistScore := 0
		artistNames := make([]string, 0, len(song.Artists))
		for _, item := range song.Artists {
			artistScore = max(artistScore, similarityScore(expectedArtist, normalizeArtistName(item.Name)))
			artistNames = append(artistNames, item.Name)
		}

		matches = append(matches, lyricCandidate{
			ID:     song.ID,
			Name:   song.Name,
			Artist: strings.Join(artistNames, ", "),
			Score:  score*3 + artistScore*2,
			Kind:   lyricSourceNetease,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches, nil
}

func addNetEaseHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Referer", "https://music.163.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Cookie", "os=pc; appver=8.10.35")
}

func fetchOnlineLyrics(ctx context.Context, netease *neteaseClient, lrclib *lrclibClient, track, artist string) (lyricDocument, error) {
	type result struct {
		doc lyricDocument
		err error
	}

	neteaseCh := make(chan result, 1)
	lrclibCh := make(chan result, 1)

	go func() {
		nctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		defer cancel()
		doc, err := netease.fetchLyrics(nctx, track, artist)
		neteaseCh <- result{doc, err}
	}()

	go func() {
		doc, err := lrclib.fetch(ctx, track, artist)
		lrclibCh <- result{doc, err}
	}()

	neteaseRes := <-neteaseCh
	lrclibRes := <-lrclibCh
	return mergeOnlineDocs(neteaseRes.doc, neteaseRes.err, lrclibRes.doc, lrclibRes.err)
}

func mergeOnlineDocs(neteaseDoc lyricDocument, neteaseErr error, lrclibDoc lyricDocument, lrclibErr error) (lyricDocument, error) {
	candidates := append([]lyricCandidate{}, neteaseDoc.Candidates...)
	candidates = append(candidates, lrclibDoc.Candidates...)

	if neteaseErr == nil && len(neteaseDoc.Lines) > 0 {
		neteaseDoc.Candidates = candidates
		return neteaseDoc, nil
	}

	if lrclibErr == nil && len(lrclibDoc.Lines) > 0 {
		lrclibDoc.Candidates = candidates
		lrclibDoc.SourceIndex = len(neteaseDoc.Candidates) + lrclibDoc.SourceIndex
		return lrclibDoc, nil
	}

	if len(candidates) > 0 {
		return lyricDocument{Candidates: candidates}, fmt.Errorf("no timed lyrics in online candidates")
	}

	return lyricDocument{}, fmt.Errorf("netease: %v; lrclib: %v", errString(neteaseErr), errString(lrclibErr))
}

func errString(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}

func fetchCandidateLyrics(ctx context.Context, netease *neteaseClient, lrclib *lrclibClient, track, artist string, current lyricDocument, index int) (lyricDocument, error) {
	if index < 0 || index >= len(current.Candidates) {
		return lyricDocument{}, fmt.Errorf("no lyric candidate at %d", index)
	}

	candidate := current.Candidates[index]
	if candidate.Kind == lyricSourceLRCLIB {
		if strings.TrimSpace(candidate.Synced) == "" {
			synced, err := lrclib.syncedByID(ctx, candidate.ID)
			if err != nil {
				return lyricDocument{}, err
			}
			current.Candidates[index].Synced = synced
		}
		doc, err := documentFromCandidate(track, artist, current.Candidates, index)
		if err != nil {
			return lyricDocument{}, err
		}
		return preserveOnlineMeta(doc, current), nil
	}

	doc, err := netease.fetchLyricsForCandidate(ctx, track, artist, current.Candidates, index)
	if err != nil {
		return lyricDocument{}, err
	}
	return preserveOnlineMeta(doc, current), nil
}

func preserveOnlineMeta(next, current lyricDocument) lyricDocument {
	next.HadBuiltin = current.HadBuiltin
	next.BuiltinLines = current.BuiltinLines
	next.BuiltinUntimed = current.BuiltinUntimed
	if next.SourceKind == "" {
		next.SourceKind = lyricSourceNetease
	}
	return next
}

func resolveLyrics(track, artist, builtin string, online lyricDocument, onlineOK bool) lyricDocument {
	track = strings.TrimSpace(track)
	artist = strings.TrimSpace(artist)
	builtinLines := parseLRC(builtin)

	if len(builtinLines) > 0 {
		return lyricDocument{
			Track:        track,
			Artist:       artist,
			Lines:        builtinLines,
			FetchedAt:    time.Now(),
			DisplayName:  fallbackLine(track, artist),
			SourceKind:   lyricSourceMusic,
			HadBuiltin:   true,
			BuiltinLines: builtinLines,
			BuiltinKind:  lyricSourceMusic,
		}
	}

	if onlineOK && len(online.Lines) > 0 {
		online.Track = track
		online.Artist = artist
		online.SourceKind = lyricSourceNetease
		return online
	}

	if strings.TrimSpace(builtin) != "" {
		line := firstPlainLyricLine(builtin)
		if line == "" {
			line = fallbackLine(track, artist)
		}
		plain := []lyricLine{{At: 0, Text: line}}
		return lyricDocument{
			Track:          track,
			Artist:         artist,
			Lines:          plain,
			FetchedAt:      time.Now(),
			DisplayName:    fallbackLine(track, artist),
			SourceKind:     lyricSourceMusic,
			Untimed:        true,
			HadBuiltin:     true,
			BuiltinLines:   plain,
			BuiltinUntimed: true,
			BuiltinKind:    lyricSourceMusic,
		}
	}

	return lyricDocument{
		Track:       track,
		Artist:      artist,
		DisplayName: fallbackLine(track, artist),
	}
}

func firstPlainLyricLine(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isLyricMetadataLine(line) {
			continue
		}
		return line
	}

	return ""
}

func isLyricMetadataLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}

	lower := strings.ToLower(trimmed)
	prefixes := []string{
		"作词", "作曲", "编曲", "制作", "lyricist", "composer", "arranger",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) || strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	if strings.HasPrefix(trimmed, "[") {
		end := strings.Index(trimmed, "]")
		if end > 1 {
			inner := strings.ToLower(trimmed[1:end])
			if strings.Contains(inner, ":") {
				key := strings.TrimSpace(strings.SplitN(inner, ":", 2)[0])
				switch key {
				case "ar", "ti", "al", "by", "offset", "length":
					return true
				}
			}
		}
	}

	return false
}

func parseLRC(raw string) []lyricLine {
	var lines []lyricLine

	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

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
