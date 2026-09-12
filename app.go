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
	lrclib         *lrclibClient
	cache          *lyricCache
	logger         *log.Logger
	configStore    *configStore
	config         config
	serviceManaged bool
	stateMu        sync.Mutex
	currentTrack   nowPlaying
	activeLyrics   lyricDocument
	cancel         context.CancelFunc
	quitOnce       sync.Once
	statusItem     *systray.MenuItem
	trackItem      *systray.MenuItem
	sourceItem     *systray.MenuItem
	nextSourceItem *systray.MenuItem
	reloadItem     *systray.MenuItem
	pathItem       *systray.MenuItem
	lastTitle      string
	lastTrackKey   string
	marquee        marqueeState
}

var (
	currentConfigStore    *configStore
	currentConfig         config
	currentServiceManaged bool
)

func startMenuBarApp(store *configStore, cfg config, serviceManaged bool) {
	currentConfigStore = store
	currentConfig = cfg
	currentServiceManaged = serviceManaged
	systray.Run(onReady, onExit)
}

func onReady() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	netease := newNetEaseClient()
	app := &app{
		client:         netease,
		lrclib:         newLRCLIBClient(netease.http),
		cache:          newLyricCache(),
		logger:         newAppLogger(),
		configStore:    currentConfigStore,
		config:         currentConfig,
		serviceManaged: currentServiceManaged,
		cancel:         cancel,
		marquee:        newMarqueeState(currentConfig.slotCells()),
	}

	systray.SetTooltip("Apple Music Lyrics Display")
	app.setTitle("启动中…")

	app.statusItem = systray.AddMenuItem("状态：启动中", "Current status")
	app.trackItem = systray.AddMenuItem("歌曲：-", "Current track")
	app.sourceItem = systray.AddMenuItem("歌词源：-", "Lyric provider")
	app.nextSourceItem = systray.AddMenuItem("换下一个歌词源", "Switch to the next lyric candidate")
	app.reloadItem = systray.AddMenuItem("重新获取歌词", "Reload lyrics for the current track")
	app.pathItem = systray.AddMenuItem("打开配置文件", "Open config file in Finder")
	systray.AddSeparator()
	quitTitle := "退出"
	if app.serviceManaged {
		quitTitle = "停止后台服务"
	}
	quitItem := systray.AddMenuItem(quitTitle, "Quit")

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
		if a.serviceManaged {
			if err := stopBrewLaunchAgent(); err != nil {
				a.logger.Printf("stop background service: %v", err)
			}
		}
		if a.cancel != nil {
			a.cancel()
		}
		systray.Quit()
	})
}

func (a *app) run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	scroll := time.NewTicker(marqueeTickInterval)
	defer scroll.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-scroll.C:
			a.tickMarquee()
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

	if appleDoc, appleErr := fetchAppleCatalogLyrics(ctx, nowPlaying); appleErr != nil {
		a.logger.Printf("apple catalog lyrics for %s: %v", key, appleErr)
	} else if len(appleDoc.Lines) > 0 {
		a.cache.put(key, appleDoc)
		return appleDoc
	}

	builtin, err := readBuiltinLyrics(ctx)
	if err != nil {
		a.logger.Printf("read builtin lyrics for %s: %v", key, err)
		builtin = ""
	}

	var online lyricDocument
	var fetchErr error
	if len(parseLRC(builtin)) == 0 {
		online, fetchErr = fetchOnlineLyrics(ctx, a.client, a.lrclib, nowPlaying.Track, nowPlaying.Artist)
		if fetchErr != nil {
			a.logger.Printf("fetch lyrics for %s: %v", key, fetchErr)
		}
	}

	doc := resolveLyrics(nowPlaying.Track, nowPlaying.Artist, builtin, online, fetchErr == nil && len(online.Lines) > 0)
	if len(online.Candidates) > 0 {
		doc.Candidates = online.Candidates
		if doc.SourceKind == lyricSourceNetease || doc.SourceKind == lyricSourceLRCLIB {
			doc.SourceIndex = online.SourceIndex
			doc.SourceID = online.SourceID
		}
	}
	if fetchErr != nil && doc.SourceKind == lyricSourceNone {
		doc.FetchError = fetchErr.Error()
	}

	if len(doc.Lines) > 0 {
		a.cache.put(key, doc)
	}
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

	a.sourceItem.SetTitle(a.sourceTitle(doc))
	a.refreshSourceMenu(doc)
	a.reloadItem.Enable()
}

func (a *app) renderPaused(np nowPlaying, doc lyricDocument) {
	line := currentLyric(doc.Lines, np.Position+a.config.offsetDuration())
	if line == "" {
		line = fallbackLine(np.Track, np.Artist)
	}

	a.setTitle(line)
	a.statusItem.SetTitle("状态：暂停")
	a.trackItem.SetTitle("歌曲：" + fallbackLine(np.Track, np.Artist))
	a.sourceItem.SetTitle(a.sourceTitle(doc))
	a.refreshSourceMenu(doc)
	a.reloadItem.Enable()
}

func (a *app) renderIdle(status string) {
	a.setTitle(status)
	a.statusItem.SetTitle("状态：" + status)
	a.trackItem.SetTitle("歌曲：-")
	a.sourceItem.SetTitle("歌词源：-")
	a.nextSourceItem.Disable()
	a.reloadItem.Disable()
}

func (a *app) renderError(message string) {
	a.setTitle(message)
	a.statusItem.SetTitle("状态：" + message)
}

func (a *app) setTitle(line string) {
	a.stateMu.Lock()
	a.marquee.setText(line, time.Now())
	title := a.config.titlePrefix() + a.marquee.view()
	unchanged := title == a.lastTitle
	if !unchanged {
		a.lastTitle = title
	}
	a.stateMu.Unlock()
	if unchanged {
		return
	}
	systray.SetTitle(title)
	a.lockMenuBarWidth()
}

func (a *app) tickMarquee() {
	a.stateMu.Lock()
	playing := a.currentTrack.State == statePlaying
	moved := a.marquee.advance(time.Now(), playing)
	title := a.config.titlePrefix() + a.marquee.view()
	unchanged := !moved && title == a.lastTitle
	if !unchanged {
		a.lastTitle = title
	}
	cfg := a.config
	a.stateMu.Unlock()
	if unchanged {
		return
	}
	systray.SetTitle(title)
	lockStatusItemToSlot(cfg.titlePrefix(), cfg.slotCells())
}

func (a *app) lockMenuBarWidth() {
	cfg := a.config
	cfg.normalize()
	lockStatusItemToSlot(cfg.titlePrefix(), cfg.slotCells())
}

func (a *app) handleMenuActions(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.nextSourceItem.ClickedCh:
			a.switchToNextSource(ctx)
		case <-a.reloadItem.ClickedCh:
			a.reloadLyrics(ctx)
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
	switch doc.SourceKind {
	case lyricSourceApple:
		return "歌词源：Apple Music"
	case lyricSourceMusic:
		if doc.Untimed {
			return "歌词源：Music 内置（无时间轴）"
		}
		return "歌词源：Music 内置"
	case lyricSourceNetease, lyricSourceLRCLIB:
		label := sourceKindLabel(doc.SourceKind)
		total := len(doc.Candidates)
		if total == 0 {
			return fmt.Sprintf("歌词源：%s #%d", label, doc.SourceID)
		}
		return fmt.Sprintf("歌词源：%d/%d %s #%d", doc.SourceIndex+1, total, label, doc.SourceID)
	default:
		if doc.FetchError != "" {
			return "歌词源：未命中，可重新搜索"
		}
		return "歌词源：未命中，显示歌曲信息"
	}
}

func sourceKindLabel(kind lyricSourceKind) string {
	switch kind {
	case lyricSourceLRCLIB:
		return "LRCLIB"
	case lyricSourceApple:
		return "Apple Music"
	case lyricSourceMusic:
		return "Music 内置"
	default:
		return "网易云"
	}
}

func (a *app) refreshSourceMenu(doc lyricDocument) {
	switch {
	case doc.SourceKind == lyricSourceNone && len(doc.Candidates) == 0:
		a.nextSourceItem.SetTitle("重新搜索歌词")
		a.nextSourceItem.Enable()
	case doc.SourceKind == lyricSourceNone:
		next := doc.Candidates[doc.SourceIndex]
		a.nextSourceItem.SetTitle(fmt.Sprintf("使用歌词源：%s - %s", trimForMenuBar(next.Name), trimForMenuBar(next.Artist)))
		a.nextSourceItem.Enable()
	case doc.SourceKind == lyricSourceMusic, doc.SourceKind == lyricSourceApple:
		a.nextSourceItem.SetTitle("改用在线歌词")
		a.nextSourceItem.Enable()
	case doc.HadBuiltin && (len(doc.Candidates) == 0 || doc.SourceIndex >= len(doc.Candidates)-1):
		a.nextSourceItem.SetTitle("换下一个歌词源：Music 内置")
		a.nextSourceItem.Enable()
	case len(doc.Candidates) <= 1:
		a.nextSourceItem.SetTitle("换下一个歌词源（无更多候选）")
		a.nextSourceItem.Disable()
	default:
		nextIndex := (doc.SourceIndex + 1) % len(doc.Candidates)
		next := doc.Candidates[nextIndex]
		a.nextSourceItem.SetTitle(fmt.Sprintf("换下一个歌词源：%s - %s", trimForMenuBar(next.Name), trimForMenuBar(next.Artist)))
		a.nextSourceItem.Enable()
	}
}

func (a *app) switchToNextSource(ctx context.Context) {
	currentTrack := a.currentNowPlaying()
	currentLyrics := a.currentLyrics()
	a.logger.Printf("switch source requested: track=%q artist=%q current=%s", currentTrack.Track, currentTrack.Artist, lyricDocumentSummary(currentLyrics))
	if currentTrack.Track == "" {
		a.logger.Printf("switch source ignored: no current track")
		return
	}

	if currentLyrics.SourceKind == lyricSourceNone && len(currentLyrics.Candidates) == 0 {
		a.logger.Printf("switch source: no current lyrics or candidates, reloading track=%q", trackKey(currentTrack.Track, currentTrack.Artist))
		a.reloadLyrics(ctx)
		return
	}

	var nextDoc lyricDocument
	var err error
	branch := ""

	switch {
	case currentLyrics.SourceKind == lyricSourceMusic, currentLyrics.SourceKind == lyricSourceApple:
		branch = "builtin-to-online"
		a.logger.Printf("switch source branch=%s: searching online lyrics", branch)
		nextDoc, err = fetchOnlineLyrics(ctx, a.client, a.lrclib, currentTrack.Track, currentTrack.Artist)
		if err == nil {
			nextDoc = preserveBuiltin(nextDoc, currentLyrics)
		}
	case currentLyrics.HadBuiltin && (len(currentLyrics.Candidates) == 0 || currentLyrics.SourceIndex >= len(currentLyrics.Candidates)-1):
		branch = "online-to-builtin"
		a.logger.Printf("switch source branch=%s: restoring builtin source", branch)
		nextDoc = restoreBuiltin(currentLyrics)
	default:
		if len(currentLyrics.Candidates) == 0 {
			a.logger.Printf("switch source: online source has no candidates, reloading track=%q", trackKey(currentTrack.Track, currentTrack.Artist))
			a.reloadLyrics(ctx)
			return
		}
		branch = "online-candidate"
		nextIndex := (currentLyrics.SourceIndex + 1) % len(currentLyrics.Candidates)
		if currentLyrics.SourceKind == lyricSourceNone {
			nextIndex = currentLyrics.SourceIndex
		}
		for attempt := 0; attempt < len(currentLyrics.Candidates); attempt++ {
			candidateIndex := (nextIndex + attempt) % len(currentLyrics.Candidates)
			if currentLyrics.SourceKind != lyricSourceNone && candidateIndex == currentLyrics.SourceIndex {
				continue
			}
			candidate := currentLyrics.Candidates[candidateIndex]
			a.logger.Printf("switch source branch=%s: candidate=%d/%d kind=%s id=%d name=%q artist=%q", branch, candidateIndex+1, len(currentLyrics.Candidates), candidate.Kind, candidate.ID, candidate.Name, candidate.Artist)
			nextDoc, err = fetchCandidateLyrics(ctx, a.client, a.lrclib, currentTrack.Track, currentTrack.Artist, currentLyrics, candidateIndex)
			if err == nil {
				nextDoc = preserveBuiltin(nextDoc, currentLyrics)
				break
			}
			a.logger.Printf("switch source candidate failed: candidate=%d/%d id=%d error=%v", candidateIndex+1, len(currentLyrics.Candidates), candidate.ID, err)
		}
	}

	if err != nil {
		a.logger.Printf("switch source failed: branch=%s track=%q error=%v", branch, trackKey(currentTrack.Track, currentTrack.Artist), err)
		a.statusItem.SetTitle("状态：切换歌词源失败")
		return
	}

	a.cache.put(trackKey(currentTrack.Track, currentTrack.Artist), nextDoc)
	a.storePlaybackState(currentTrack, nextDoc)
	a.lastTitle = ""
	a.renderLyric(currentTrack, nextDoc)
	a.logger.Printf("switch source succeeded: branch=%s from=%s to=%s", branch, lyricDocumentSummary(currentLyrics), lyricDocumentSummary(nextDoc))
}

func lyricDocumentSummary(doc lyricDocument) string {
	source := string(doc.SourceKind)
	if source == "" {
		source = "none"
	}
	return fmt.Sprintf("source=%s index=%d candidates=%d builtin=%t lines=%d", source, doc.SourceIndex, len(doc.Candidates), doc.HadBuiltin, len(doc.Lines))
}

func (a *app) reloadLyrics(ctx context.Context) {
	currentTrack := a.currentNowPlaying()
	if currentTrack.Track == "" {
		a.logger.Printf("reload lyrics ignored: no current track")
		return
	}
	key := trackKey(currentTrack.Track, currentTrack.Artist)
	a.logger.Printf("reload lyrics requested: track=%q artist=%q", currentTrack.Track, currentTrack.Artist)
	a.cache.delete(key)
	a.lastTrackKey = ""
	a.statusItem.SetTitle("状态：正在重新获取歌词")
	doc := a.loadLyrics(ctx, currentTrack)
	a.lastTrackKey = key
	a.storePlaybackState(currentTrack, doc)
	a.lastTitle = ""
	a.renderLyric(currentTrack, doc)
	a.logger.Printf("reload lyrics completed: track=%q result=%s", key, lyricDocumentSummary(doc))
}

func preserveBuiltin(next, current lyricDocument) lyricDocument {
	next.HadBuiltin = current.HadBuiltin || current.SourceKind == lyricSourceMusic || current.SourceKind == lyricSourceApple
	if len(current.BuiltinLines) > 0 {
		next.BuiltinLines = current.BuiltinLines
		next.BuiltinUntimed = current.BuiltinUntimed
		if current.BuiltinKind != "" {
			next.BuiltinKind = current.BuiltinKind
		} else {
			next.BuiltinKind = current.SourceKind
		}
	} else if current.SourceKind == lyricSourceMusic || current.SourceKind == lyricSourceApple {
		next.BuiltinLines = current.Lines
		next.BuiltinUntimed = current.Untimed
		next.HadBuiltin = true
		next.BuiltinKind = current.SourceKind
	}
	if next.SourceKind == "" {
		next.SourceKind = lyricSourceNetease
	}
	return next
}

func restoreBuiltin(current lyricDocument) lyricDocument {
	lines := current.BuiltinLines
	if len(lines) == 0 {
		lines = current.Lines
	}
	kind := current.BuiltinKind
	if kind == "" {
		kind = lyricSourceMusic
	}
	return lyricDocument{
		Track:          current.Track,
		Artist:         current.Artist,
		Lines:          lines,
		FetchedAt:      time.Now(),
		DisplayName:    fallbackLine(current.Track, current.Artist),
		Candidates:     current.Candidates,
		SourceKind:     kind,
		Untimed:        current.BuiltinUntimed,
		HadBuiltin:     true,
		BuiltinLines:   lines,
		BuiltinUntimed: current.BuiltinUntimed,
		BuiltinKind:    kind,
	}
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
