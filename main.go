package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
)

const (
	menuBarMaxRunes = 28
	pollInterval    = 500 * time.Millisecond
	defaultOffset   = 350 * time.Millisecond
	requestTimeout  = 6 * time.Second
	fieldSeparator  = "\x1f"
	appName         = "lyrics-display"
	defaultEmoji    = "♪"
)

var (
	version   = "dev"
	commit    = ""
	buildDate = ""
)

var lrcPattern = regexp.MustCompile(`\[(\d{2,}):(\d{2})(?:\.(\d{1,3}))?\]([^\n\r]*)`)

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
}

type lyricLine struct {
	At   time.Duration
	Text string
}

type lyricDocument struct {
	Track       string
	Artist      string
	SourceID    int64
	Lines       []lyricLine
	FetchedAt   time.Time
	DisplayName string
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

type config struct {
	ShowEmoji bool   `json:"show_emoji"`
	Emoji     string `json:"emoji"`
	OffsetMS  int    `json:"offset_ms"`
}

func defaultConfig() config {
	return config{
		ShowEmoji: true,
		Emoji:     defaultEmoji,
		OffsetMS:  int(defaultOffset / time.Millisecond),
	}
}

type configStore struct {
	path string
	mu   sync.Mutex
}

func newConfigStore() (*configStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	return &configStore{
		path: filepath.Join(configDir, appName, "config.json"),
	}, nil
}

func (s *configStore) pathString() string {
	return s.path
}

func (s *configStore) ensureDir() error {
	return os.MkdirAll(filepath.Dir(s.path), 0o755)
}

func (s *configStore) load() (config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := defaultConfig()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	cfg.normalize()
	return cfg, nil
}

func (s *configStore) save(cfg config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg.normalize()

	if err := s.ensureDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')
	return os.WriteFile(s.path, data, 0o644)
}

func (c *config) normalize() {
	if strings.TrimSpace(c.Emoji) == "" {
		c.Emoji = defaultEmoji
	}
	if c.OffsetMS < -5000 {
		c.OffsetMS = -5000
	}
	if c.OffsetMS > 5000 {
		c.OffsetMS = 5000
	}
}

func (c config) titlePrefix() string {
	if !c.ShowEmoji {
		return ""
	}
	return strings.TrimSpace(c.Emoji) + " "
}

func (c config) offsetDuration() time.Duration {
	return time.Duration(c.OffsetMS) * time.Millisecond
}

type app struct {
	client       *neteaseClient
	cache        *lyricCache
	logger       *log.Logger
	configStore  *configStore
	config       config
	cancel       context.CancelFunc
	quitOnce     sync.Once
	statusItem   *systray.MenuItem
	trackItem    *systray.MenuItem
	sourceItem   *systray.MenuItem
	offsetItem   *systray.MenuItem
	emojiItem    *systray.MenuItem
	fasterItem   *systray.MenuItem
	slowerItem   *systray.MenuItem
	resetItem    *systray.MenuItem
	pathItem     *systray.MenuItem
	lastTitle    string
	lastTrackKey string
}

func main() {
	store, err := newConfigStore()
	if err != nil {
		log.Fatalf("resolve config path: %v", err)
	}

	cfg, err := store.load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if handled := handleCLI(os.Args[1:], os.Stdout, store, cfg); handled {
		return
	}

	startMenuBarApp(store, cfg)
}

func handleCLI(args []string, stdout io.Writer, store *configStore, cfg config) bool {
	fs := flag.NewFlagSet("lyrics-display", flag.ContinueOnError)
	fs.SetOutput(stdout)

	showVersion := fs.Bool("version", false, "print version information")
	showHelp := fs.Bool("help", false, "show help")
	fs.BoolVar(showHelp, "h", false, "show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stdout)
		printUsage(stdout)
		return true
	}

	if *showHelp {
		printUsage(stdout)
		return true
	}

	if *showVersion {
		fmt.Fprintln(stdout, versionString())
		return true
	}

	if fs.NArg() == 0 {
		return false
	}

	switch fs.Arg(0) {
	case "version":
		fmt.Fprintln(stdout, versionString())
		return true
	case "status":
		return handleStatus(stdout, store, cfg)
	case "config":
		return handleConfigCommand(stdout, store, cfg, fs.Args()[1:])
	case "offset":
		return handleOffsetCommand(stdout, store, cfg, fs.Args()[1:])
	}

	return false
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `lyrics-display

Apple Music menu bar lyrics for macOS.

Usage:
  lyrics-display                                        Start the menu bar app
  lyrics-display --help                                 Show this help text
  lyrics-display --version                              Print version information
  lyrics-display status                                 Show current config and runtime hints
  lyrics-display config show                            Print current config JSON
  lyrics-display config path                            Print config file path
  lyrics-display config init                            Create the default config file
  lyrics-display config set emoji on|off                Enable or disable the menu bar emoji
  lyrics-display config set emoji-char "♪"             Set the emoji or prefix symbol
  lyrics-display config set offset-ms 450               Set lyric sync offset in milliseconds
  lyrics-display offset +100                            Delay lyric matching by 100ms
  lyrics-display offset -100                            Advance lyric matching by 100ms
  lyrics-display offset set 350                         Set lyric offset directly
`)
}

func versionString() string {
	parts := []string{version}
	if commit != "" {
		parts = append(parts, "commit="+commit)
	}
	if buildDate != "" {
		parts = append(parts, "built="+buildDate)
	}
	return strings.Join(parts, " ")
}

func startMenuBarApp(store *configStore, cfg config) {
	currentConfigStore = store
	currentConfig = cfg
	systray.Run(onReady, onExit)
}

var (
	currentConfigStore *configStore
	currentConfig      config
)

func onReady() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	app := &app{
		client:      newNetEaseClient(),
		cache:       newLyricCache(),
		logger:      log.New(os.Stdout, "[lyrics-display] ", log.LstdFlags),
		configStore: currentConfigStore,
		config:      currentConfig,
		cancel:      cancel,
	}

	systray.SetTooltip("Apple Music Lyrics Display")
	systray.SetTitle(app.config.titlePrefix() + "启动中…")

	app.statusItem = systray.AddMenuItem("状态：启动中", "Current status")
	app.trackItem = systray.AddMenuItem("歌曲：-", "Current track")
	app.sourceItem = systray.AddMenuItem("歌词源：网易云音乐", "Lyric provider")
	app.offsetItem = systray.AddMenuItem("", "Current lyric offset")
	app.emojiItem = systray.AddMenuItem("", "Toggle emoji prefix")
	app.fasterItem = systray.AddMenuItem("歌词提前 100ms", "Advance lyric sync")
	app.slowerItem = systray.AddMenuItem("歌词延后 100ms", "Delay lyric sync")
	app.resetItem = systray.AddMenuItem("重置偏移为 350ms", "Reset lyric offset")
	app.pathItem = systray.AddMenuItem("", "Config file path")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("退出", "Quit")

	app.refreshConfigMenu()

	go func() {
		select {
		case <-quitItem.ClickedCh:
			app.quit()
		case <-ctx.Done():
			app.quit()
		}
	}()

	go app.handleMenuActions(ctx)
	go app.run(ctx)
}

func onExit() {}

func (a *app) quit() {
	a.quitOnce.Do(func() {
		if a.cancel != nil {
			a.cancel()
		}
		systray.Quit()
	})
}

func (a *app) run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var activeLyrics lyricDocument

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			nowPlaying, err := readNowPlaying(ctx)
			if err != nil {
				a.logger.Printf("read player state: %v", err)
				a.renderError("未获取到 Apple Music 状态")
				continue
			}

			switch nowPlaying.State {
			case stateStopped:
				activeLyrics = lyricDocument{}
				a.lastTrackKey = ""
				a.renderIdle("Apple Music 未播放")
			case statePaused:
				if activeLyrics.Track == "" || a.lastTrackKey != trackKey(nowPlaying.Track, nowPlaying.Artist) {
					activeLyrics = a.loadLyrics(ctx, nowPlaying)
				}
				a.renderPaused(nowPlaying, activeLyrics)
			case statePlaying:
				if a.lastTrackKey != trackKey(nowPlaying.Track, nowPlaying.Artist) {
					activeLyrics = a.loadLyrics(ctx, nowPlaying)
					a.lastTrackKey = trackKey(nowPlaying.Track, nowPlaying.Artist)
				}
				a.renderLyric(nowPlaying, activeLyrics)
			default:
				a.renderIdle("等待 Apple Music")
			}
		}
	}
}

func (a *app) loadLyrics(ctx context.Context, nowPlaying nowPlaying) lyricDocument {
	key := trackKey(nowPlaying.Track, nowPlaying.Artist)
	if cached, ok := a.cache.get(key); ok {
		return cached
	}

	doc, err := a.client.fetchLyrics(ctx, nowPlaying.Track, nowPlaying.Artist)
	if err != nil {
		a.logger.Printf("fetch lyrics for %s: %v", key, err)
		return lyricDocument{
			Track:       nowPlaying.Track,
			Artist:      nowPlaying.Artist,
			DisplayName: fallbackLine(nowPlaying.Track, nowPlaying.Artist),
		}
	}

	a.cache.put(key, doc)
	return doc
}

func (a *app) renderLyric(np nowPlaying, doc lyricDocument) {
	line := currentLyric(doc.Lines, np.Position+a.config.offsetDuration())
	if line == "" {
		line = fallbackLine(np.Track, np.Artist)
	}

	a.setTitle(line)
	a.statusItem.SetTitle("状态：播放中")
	a.trackItem.SetTitle("歌曲：" + fallbackLine(np.Track, np.Artist))

	if doc.SourceID > 0 {
		a.sourceItem.SetTitle(fmt.Sprintf("歌词源：网易云 #%d", doc.SourceID))
	} else {
		a.sourceItem.SetTitle("歌词源：未命中，显示歌曲信息")
	}
}

func (a *app) renderPaused(np nowPlaying, doc lyricDocument) {
	line := currentLyric(doc.Lines, np.Position+a.config.offsetDuration())
	if line == "" {
		line = fallbackLine(np.Track, np.Artist)
	}

	a.setTitle("暂停 | " + line)
	a.statusItem.SetTitle("状态：暂停")
	a.trackItem.SetTitle("歌曲：" + fallbackLine(np.Track, np.Artist))
}

func (a *app) renderIdle(status string) {
	a.setTitle(status)
	a.statusItem.SetTitle("状态：" + status)
	a.trackItem.SetTitle("歌曲：-")
	a.sourceItem.SetTitle("歌词源：网易云音乐")
}

func (a *app) renderError(message string) {
	a.setTitle(message)
	a.statusItem.SetTitle("状态：" + message)
}

func (a *app) setTitle(line string) {
	title := a.config.titlePrefix() + trimForMenuBar(line)
	if title == a.lastTitle {
		return
	}

	systray.SetTitle(title)
	a.lastTitle = title
}

func readNowPlaying(ctx context.Context) (nowPlaying, error) {
	if !musicAppRunning(ctx) {
		return nowPlaying{State: stateStopped}, nil
	}

	script := `
on sanitizeText(value)
	set textValue to value as text
	set textValue to my replaceText(return, " ", textValue)
	set textValue to my replaceText(linefeed, " ", textValue)
	set textValue to my replaceText(character id 31, " ", textValue)
	return textValue
end sanitizeText

on replaceText(findText, replaceText, subject)
	set AppleScript's text item delimiters to findText
	set textItems to every text item of subject
	set AppleScript's text item delimiters to replaceText
	set subject to textItems as text
	set AppleScript's text item delimiters to ""
	return subject
end replaceText

tell application "Music"
	set currentState to player state as text
	if currentState is "stopped" then
		return "stopped" & character id 31 & "" & character id 31 & "" & character id 31 & "" & character id 31 & "0"
	end if

	set trackName to my sanitizeText(name of current track)
	set artistName to my sanitizeText(artist of current track)
	set albumName to my sanitizeText(album of current track)
	set playerPosition to player position

	return currentState & character id 31 & trackName & character id 31 & artistName & character id 31 & albumName & character id 31 & (playerPosition as text)
end tell
`

	cmd := exec.CommandContext(ctx, "osascript", "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nowPlaying{}, fmt.Errorf("osascript failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nowPlaying{}, err
	}

	parts := strings.Split(strings.TrimSpace(string(out)), fieldSeparator)
	if len(parts) != 5 {
		return nowPlaying{}, fmt.Errorf("unexpected player payload: %q", string(out))
	}

	seconds, err := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
	if err != nil {
		return nowPlaying{}, fmt.Errorf("parse player position: %w", err)
	}

	return nowPlaying{
		State:    playerState(strings.TrimSpace(parts[0])),
		Track:    strings.TrimSpace(parts[1]),
		Artist:   strings.TrimSpace(parts[2]),
		Album:    strings.TrimSpace(parts[3]),
		Position: time.Duration(seconds * float64(time.Second)),
	}, nil
}

func (a *app) handleMenuActions(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.emojiItem.ClickedCh:
			a.config.ShowEmoji = !a.config.ShowEmoji
			a.persistConfig()
			a.refreshConfigMenu()
			a.lastTitle = ""
		case <-a.fasterItem.ClickedCh:
			a.config.OffsetMS -= 100
			a.persistConfig()
			a.refreshConfigMenu()
		case <-a.slowerItem.ClickedCh:
			a.config.OffsetMS += 100
			a.persistConfig()
			a.refreshConfigMenu()
		case <-a.resetItem.ClickedCh:
			a.config.OffsetMS = int(defaultOffset / time.Millisecond)
			a.persistConfig()
			a.refreshConfigMenu()
		case <-a.pathItem.ClickedCh:
			a.logger.Printf("config path: %s", a.configStore.pathString())
		}
	}
}

func (a *app) persistConfig() {
	a.config.normalize()
	if err := a.configStore.save(a.config); err != nil {
		a.logger.Printf("save config: %v", err)
	}
}

func (a *app) refreshConfigMenu() {
	a.config.normalize()
	a.offsetItem.SetTitle(fmt.Sprintf("歌词偏移：%dms", a.config.OffsetMS))
	if a.config.ShowEmoji {
		a.emojiItem.SetTitle("Emoji：开启")
	} else {
		a.emojiItem.SetTitle("Emoji：关闭")
	}
	a.pathItem.SetTitle("配置文件：" + a.configStore.pathString())
}

func musicAppRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "pgrep", "-x", "Music")
	return cmd.Run() == nil
}

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

func handleStatus(stdout io.Writer, store *configStore, cfg config) bool {
	cfg.normalize()
	fmt.Fprintf(stdout, "version: %s\n", versionString())
	fmt.Fprintf(stdout, "config: %s\n", store.pathString())
	fmt.Fprintf(stdout, "emoji: %t (%s)\n", cfg.ShowEmoji, cfg.Emoji)
	fmt.Fprintf(stdout, "offset_ms: %d\n", cfg.OffsetMS)
	fmt.Fprintf(stdout, "music_running: %t\n", musicAppRunning(context.Background()))
	fmt.Fprintln(stdout, "note: menu bar text color and font are controlled by macOS and are not configurable in this build.")
	return true
}

func handleConfigCommand(stdout io.Writer, store *configStore, cfg config, args []string) bool {
	if len(args) == 0 {
		printUsage(stdout)
		return true
	}

	switch args[0] {
	case "show":
		cfg.normalize()
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			fmt.Fprintf(stdout, "marshal config: %v\n", err)
			return true
		}
		fmt.Fprintf(stdout, "%s\n", data)
	case "path":
		fmt.Fprintln(stdout, store.pathString())
	case "init":
		if err := store.save(cfg); err != nil {
			fmt.Fprintf(stdout, "save config: %v\n", err)
			return true
		}
		fmt.Fprintf(stdout, "initialized config at %s\n", store.pathString())
	case "set":
		if len(args) < 3 {
			fmt.Fprintln(stdout, "usage: lyrics-display config set <emoji|emoji-char|offset-ms> <value>")
			return true
		}
		switch args[1] {
		case "emoji":
			value, ok := parseOnOff(args[2])
			if !ok {
				fmt.Fprintln(stdout, "emoji expects on or off")
				return true
			}
			cfg.ShowEmoji = value
		case "emoji-char":
			cfg.Emoji = args[2]
		case "offset-ms":
			value, err := strconv.Atoi(args[2])
			if err != nil {
				fmt.Fprintln(stdout, "offset-ms expects an integer")
				return true
			}
			cfg.OffsetMS = value
		default:
			fmt.Fprintln(stdout, "supported keys: emoji, emoji-char, offset-ms")
			return true
		}

		if err := store.save(cfg); err != nil {
			fmt.Fprintf(stdout, "save config: %v\n", err)
			return true
		}
		fmt.Fprintf(stdout, "updated config at %s\n", store.pathString())
	default:
		printUsage(stdout)
	}
	return true
}

func handleOffsetCommand(stdout io.Writer, store *configStore, cfg config, args []string) bool {
	if len(args) == 0 {
		fmt.Fprintln(stdout, "usage: lyrics-display offset [+100|-100|set 350]")
		return true
	}

	switch args[0] {
	case "set":
		if len(args) < 2 {
			fmt.Fprintln(stdout, "usage: lyrics-display offset set <milliseconds>")
			return true
		}
		value, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintln(stdout, "offset set expects an integer")
			return true
		}
		cfg.OffsetMS = value
	default:
		delta, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintln(stdout, "offset expects +N, -N, or set N")
			return true
		}
		cfg.OffsetMS += delta
	}

	if err := store.save(cfg); err != nil {
		fmt.Fprintf(stdout, "save config: %v\n", err)
		return true
	}

	updated, err := store.load()
	if err != nil {
		fmt.Fprintf(stdout, "reload config: %v\n", err)
		return true
	}
	fmt.Fprintf(stdout, "offset_ms: %d\n", updated.OffsetMS)
	return true
}

func parseOnOff(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "true", "1":
		return true, true
	case "off", "false", "0":
		return false, true
	default:
		return false, false
	}
}
