package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
)

type app struct {
	client         *neteaseClient
	cache          *lyricCache
	logger         *log.Logger
	configStore    *configStore
	config         config
	stateMu        sync.Mutex
	currentTrack   nowPlaying
	activeLyrics   lyricDocument
	cancel         context.CancelFunc
	quitOnce       sync.Once
	statusItem     *systray.MenuItem
	trackItem      *systray.MenuItem
	sourceItem     *systray.MenuItem
	nextSourceItem *systray.MenuItem
	pathItem       *systray.MenuItem
	lastTitle      string
	lastTrackKey   string
}

var (
	currentConfigStore *configStore
	currentConfig      config
)

func startMenuBarApp(store *configStore, cfg config) {
	currentConfigStore = store
	currentConfig = cfg
	systray.Run(onReady, onExit)
}

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
	app.nextSourceItem = systray.AddMenuItem("换下一个歌词源", "Switch to the next lyric candidate")
	app.pathItem = systray.AddMenuItem("打开配置文件", "Open config file in Finder")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("退出", "Quit")

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
				a.stateMu.Lock()
				a.activeLyrics = lyricDocument{}
				a.currentTrack = nowPlaying
				a.stateMu.Unlock()
				a.lastTrackKey = ""
				a.renderIdle("Apple Music 未播放")
			case statePaused:
				activeLyrics := a.currentLyrics()
				if activeLyrics.Track == "" || a.lastTrackKey != trackKey(nowPlaying.Track, nowPlaying.Artist) {
					activeLyrics = a.loadLyrics(ctx, nowPlaying)
				}
				a.storePlaybackState(nowPlaying, activeLyrics)
				a.renderPaused(nowPlaying, activeLyrics)
			case statePlaying:
				if a.lastTrackKey != trackKey(nowPlaying.Track, nowPlaying.Artist) {
					activeLyrics := a.loadLyrics(ctx, nowPlaying)
					a.lastTrackKey = trackKey(nowPlaying.Track, nowPlaying.Artist)
					a.storePlaybackState(nowPlaying, activeLyrics)
					a.renderLyric(nowPlaying, activeLyrics)
					continue
				}
				activeLyrics := a.currentLyrics()
				a.storePlaybackState(nowPlaying, activeLyrics)
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
		a.sourceItem.SetTitle(a.sourceTitle(doc))
	} else {
		a.sourceItem.SetTitle("歌词源：未命中，显示歌曲信息")
	}
	a.refreshSourceMenu(doc)
}

func (a *app) renderPaused(np nowPlaying, doc lyricDocument) {
	line := currentLyric(doc.Lines, np.Position+a.config.offsetDuration())
	if line == "" {
		line = fallbackLine(np.Track, np.Artist)
	}

	a.setTitle("暂停 | " + line)
	a.statusItem.SetTitle("状态：暂停")
	a.trackItem.SetTitle("歌曲：" + fallbackLine(np.Track, np.Artist))
	if doc.SourceID > 0 {
		a.sourceItem.SetTitle(a.sourceTitle(doc))
	}
	a.refreshSourceMenu(doc)
}

func (a *app) renderIdle(status string) {
	a.setTitle(status)
	a.statusItem.SetTitle("状态：" + status)
	a.trackItem.SetTitle("歌曲：-")
	a.sourceItem.SetTitle("歌词源：网易云音乐")
	a.nextSourceItem.Disable()
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

func (a *app) handleMenuActions(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.nextSourceItem.ClickedCh:
			a.switchToNextSource(ctx)
		case <-a.pathItem.ClickedCh:
			if err := a.openConfigInFinder(); err != nil {
				a.logger.Printf("open config in finder: %v", err)
				a.statusItem.SetTitle("状态：打开配置文件失败")
			}
		}
	}
}

func (a *app) persistConfig() {
	a.config.normalize()
	if err := a.configStore.save(a.config); err != nil {
		a.logger.Printf("save config: %v", err)
	}
}

func (a *app) storePlaybackState(track nowPlaying, doc lyricDocument) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.currentTrack = track
	a.activeLyrics = doc
}

func (a *app) currentLyrics() lyricDocument {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.activeLyrics
}

func (a *app) currentNowPlaying() nowPlaying {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.currentTrack
}

func (a *app) sourceTitle(doc lyricDocument) string {
	total := len(doc.Candidates)
	if total == 0 {
		return fmt.Sprintf("歌词源：网易云 #%d", doc.SourceID)
	}
	return fmt.Sprintf("歌词源：%d/%d 网易云 #%d", doc.SourceIndex+1, total, doc.SourceID)
}

func (a *app) refreshSourceMenu(doc lyricDocument) {
	if len(doc.Candidates) <= 1 {
		a.nextSourceItem.SetTitle("换下一个歌词源（无更多候选）")
		a.nextSourceItem.Disable()
		return
	}

	nextIndex := (doc.SourceIndex + 1) % len(doc.Candidates)
	next := doc.Candidates[nextIndex]
	a.nextSourceItem.SetTitle(fmt.Sprintf("换下一个歌词源：%s - %s", trimForMenuBar(next.Name), trimForMenuBar(next.Artist)))
	a.nextSourceItem.Enable()
}

func (a *app) switchToNextSource(ctx context.Context) {
	currentTrack := a.currentNowPlaying()
	currentLyrics := a.currentLyrics()
	if currentTrack.Track == "" || len(currentLyrics.Candidates) <= 1 {
		return
	}

	nextDoc, err := a.client.fetchNextLyrics(ctx, currentTrack.Track, currentTrack.Artist, currentLyrics)
	if err != nil {
		a.logger.Printf("switch source: %v", err)
		a.statusItem.SetTitle("状态：切换歌词源失败")
		return
	}

	a.cache.put(trackKey(currentTrack.Track, currentTrack.Artist), nextDoc)
	a.storePlaybackState(currentTrack, nextDoc)
	a.lastTitle = ""
	a.renderLyric(currentTrack, nextDoc)
}

func (a *app) openConfigInFinder() error {
	if _, err := os.Stat(a.configStore.pathString()); err != nil {
		if os.IsNotExist(err) {
			if err := a.configStore.save(a.config); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return exec.Command("open", "-R", a.configStore.pathString()).Run()
}
