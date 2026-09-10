package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const appleLyricsMatchThreshold = 200

type appleLyricsPayload struct {
	LyricsID string `json:"lyricsId"`
	TTML     string `json:"ttml"`
	Status   int    `json:"status"`
}

type appleCacheEntry struct {
	RequestKey string `json:"request_key"`
	IsDataOnFS int    `json:"isDataOnFS"`
	Data       string `json:"data"`
	TimeStamp  string `json:"time_stamp"`
}

type itunesSong struct {
	ID       int64
	Name     string
	Artist   string
	Album    string
	Duration time.Duration
}

var (
	itunesLookupCache sync.Map
	appleHTTP         = &http.Client{Timeout: 5 * time.Second}
)

func fetchAppleCatalogLyrics(ctx context.Context, np nowPlaying) (lyricDocument, error) {
	if strings.TrimSpace(np.Track) == "" {
		return lyricDocument{}, fmt.Errorf("no current track")
	}

	entries, err := listAppleTTMLCache()
	if err != nil {
		return lyricDocument{}, err
	}
	if len(entries) == 0 {
		return lyricDocument{}, fmt.Errorf("Apple Music 歌词缓存为空，请在「音乐」里打开一次歌词面板")
	}

	wantedIDs := map[string]int{}
	if ids, searchErr := searchAppleSongIDs(ctx, np.Track, np.Artist); searchErr == nil {
		for i, id := range ids {
			wantedIDs[id] = len(ids) - i
		}
	}

	type scored struct {
		entry appleCacheEntry
		meta  itunesSong
		score int
	}
	var best *scored

	for _, entry := range entries {
		songID := appleSongIDFromURL(entry.RequestKey)
		if songID == "" {
			continue
		}
		meta, lookupErr := lookupAppleSong(ctx, songID)
		if lookupErr != nil {
			if _, ok := wantedIDs[songID]; !ok {
				continue
			}
			meta = itunesSong{ID: parseInt64(songID), Name: np.Track, Artist: np.Artist}
		}
		score := appleMatchScore(np, meta)
		if bonus, ok := wantedIDs[songID]; ok {
			score += 80 + bonus
		}
		if best == nil || score > best.score {
			best = &scored{entry: entry, meta: meta, score: score}
		}
	}

	if best == nil || best.score < appleLyricsMatchThreshold {
		return lyricDocument{}, fmt.Errorf("Apple Music 歌词缓存未命中当前歌曲")
	}

	ttml, err := readAppleTTML(best.entry)
	if err != nil {
		return lyricDocument{}, err
	}
	lines := parseTTML(ttml)
	if len(lines) == 0 {
		return lyricDocument{}, fmt.Errorf("Apple Music TTML 没有时间轴歌词")
	}

	return lyricDocument{
		Track:        np.Track,
		Artist:       np.Artist,
		SourceID:     best.meta.ID,
		Lines:        lines,
		FetchedAt:    time.Now(),
		DisplayName:  fallbackLine(best.meta.Name, best.meta.Artist),
		SourceKind:   lyricSourceApple,
		HadBuiltin:   true,
		BuiltinLines: lines,
		BuiltinKind:  lyricSourceApple,
	}, nil
}

func listAppleTTMLCache() ([]appleCacheEntry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	src := filepath.Join(home, "Library", "Caches", "com.apple.Music", "Cache.db")
	if _, err := os.Stat(src); err != nil {
		return nil, fmt.Errorf("Apple Music cache not found")
	}

	tmpDir, err := os.MkdirTemp("", "lyrics-display-music-cache-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	dst := filepath.Join(tmpDir, "Cache.db")
	if err := copyFile(src, dst); err != nil {
		return nil, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecar := src + suffix
		if _, err := os.Stat(sidecar); err == nil {
			_ = copyFile(sidecar, dst+suffix)
		}
	}

	query := `SELECT r.request_key AS request_key, d.isDataOnFS AS isDataOnFS, CAST(d.receiver_data AS TEXT) AS data, r.time_stamp AS time_stamp
FROM cfurl_cache_response r
JOIN cfurl_cache_receiver_data d USING(entry_ID)
WHERE r.request_key LIKE '%ttmlLyrics%'
ORDER BY r.time_stamp DESC
LIMIT 40;`

	cmd := exec.Command("sqlite3", "-json", dst, query)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("sqlite3: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	if strings.TrimSpace(string(out)) == "" {
		return nil, nil
	}

	var rows []appleCacheEntry
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func readAppleTTML(entry appleCacheEntry) (string, error) {
	var body []byte
	if entry.IsDataOnFS == 1 {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		uuid := strings.TrimSpace(entry.Data)
		path := filepath.Join(home, "Library", "Caches", "com.apple.Music", "fsCachedData", uuid)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		body = data
	} else {
		body = []byte(entry.Data)
	}

	var payload appleLyricsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.TTML) == "" {
		return "", fmt.Errorf("cached lyrics payload has no ttml")
	}
	return payload.TTML, nil
}

func searchAppleSongIDs(ctx context.Context, track, artist string) ([]string, error) {
	query := strings.TrimSpace(track + " " + artist)
	if query == "" {
		return nil, fmt.Errorf("empty search")
	}

	var ids []string
	seen := map[string]bool{}
	for _, country := range []string{"cn", "hk", "tw", "us"} {
		endpoint := "https://itunes.apple.com/search?term=" + url.QueryEscape(query) + "&entity=song&limit=8&country=" + country
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "lyrics-display/0.2")
		resp, err := appleHTTP.Do(req)
		if err != nil {
			continue
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				io.Copy(io.Discard, io.LimitReader(resp.Body, 256))
				return
			}
			var payload struct {
				Results []struct {
					TrackID    int64  `json:"trackId"`
					TrackName  string `json:"trackName"`
					ArtistName string `json:"artistName"`
				} `json:"results"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				return
			}
			for _, item := range payload.Results {
				id := strconv.FormatInt(item.TrackID, 10)
				if seen[id] {
					continue
				}
				seen[id] = true
				ids = append(ids, id)
			}
		}()
		if len(ids) > 0 {
			break
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("itunes search returned no songs")
	}
	return ids, nil
}

func lookupAppleSong(ctx context.Context, songID string) (itunesSong, error) {
	if cached, ok := itunesLookupCache.Load(songID); ok {
		if cached == nil {
			return itunesSong{}, fmt.Errorf("itunes lookup miss %s", songID)
		}
		return cached.(itunesSong), nil
	}

	var found itunesSong
	var ok bool
	for _, country := range []string{"cn", "hk", "tw", "us", ""} {
		endpoint := "https://itunes.apple.com/lookup?id=" + url.QueryEscape(songID) + "&entity=song"
		if country != "" {
			endpoint += "&country=" + country
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "lyrics-display/0.2")
		resp, err := appleHTTP.Do(req)
		if err != nil {
			continue
		}
		song, err := decodeItunesLookup(resp, songID)
		resp.Body.Close()
		if err != nil {
			continue
		}
		found = song
		ok = true
		break
	}
	if !ok {
		itunesLookupCache.Store(songID, nil)
		return itunesSong{}, fmt.Errorf("itunes lookup miss %s", songID)
	}
	itunesLookupCache.Store(songID, found)
	return found, nil
}

func decodeItunesLookup(resp *http.Response, songID string) (itunesSong, error) {
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 256))
		return itunesSong{}, fmt.Errorf("itunes lookup status %s", resp.Status)
	}
	var payload struct {
		Results []struct {
			TrackID         int64  `json:"trackId"`
			TrackName       string `json:"trackName"`
			ArtistName      string `json:"artistName"`
			CollectionName  string `json:"collectionName"`
			TrackTimeMillis int64  `json:"trackTimeMillis"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return itunesSong{}, err
	}
	want, _ := strconv.ParseInt(songID, 10, 64)
	for _, item := range payload.Results {
		if item.TrackID == want || want == 0 {
			return itunesSong{
				ID:       item.TrackID,
				Name:     item.TrackName,
				Artist:   item.ArtistName,
				Album:    item.CollectionName,
				Duration: time.Duration(item.TrackTimeMillis) * time.Millisecond,
			}, nil
		}
	}
	return itunesSong{}, fmt.Errorf("no lookup result")
}

func appleMatchScore(np nowPlaying, meta itunesSong) int {
	title := similarityScore(normalizeSongName(np.Track), normalizeSongName(meta.Name))
	artist := similarityScore(normalizeArtistName(np.Artist), normalizeArtistName(meta.Artist))
	score := title*3 + artist*2
	if np.Duration > 0 && meta.Duration > 0 {
		diff := np.Duration - meta.Duration
		if diff < 0 {
			diff = -diff
		}
		if diff <= 2*time.Second {
			score += 25
		} else if diff > 6*time.Second {
			score -= 40
		}
	}
	return score
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func parseInt64(raw string) int64 {
	value, _ := strconv.ParseInt(raw, 10, 64)
	return value
}
