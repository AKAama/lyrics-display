# Changelog

## v0.2.0

Apple Music lyrics, native app packaging, and menu bar slot polish.

### Added

- Prefer Apple Music built-in lyrics when they contain a timeline, then fall back to NetEase
- Fall back to LRCLIB when NetEase is blocked or returns no timed lyrics
- Use unsynced Music lyrics when no timed source is available
- Menu action to switch from built-in lyrics to online lyrics, and back
- Menu action to reload lyrics for the current track
- Configurable menu bar slot width via `slot_width`
- Keep the menu bar lyric slot at a fixed pixel width while scrolling mixed Chinese/English lines, without rewriting Latin letters as fullwidth glyphs
- Read official Apple Music timed lyrics from Music.app's local TTML cache
- Native `.app` / DMG packaging for drag-and-drop install
- `--service` mode so Homebrew services can be stopped from the menu bar

### Fixed

- Empty lyric misses are no longer cached, so a failed first fetch cannot stick forever
- The source menu stays usable after a miss, with retry and candidate switching
- Quitting from the menu while launched by `brew services` now unloads the LaunchAgent instead of being restarted by `keep_alive`

## v0.1.3

Configuration workflow cleanup release.

### Changed

- Simplified the menu bar menu by removing direct emoji and offset toggles
- Moved advanced customization back to the config file as the primary editing path
- Added a menu action to reveal the config file directly in Finder

### Notes

- Day-to-day controls stay lightweight in the menu bar
- Emoji and lyric offset are still supported, but are now intended to be edited through the config file

## v0.1.2

Lyric source correction release.

### Added

- Manual menu action to switch to the next lyric source candidate
- Candidate tracking so users can cycle through alternative NetEase matches for the current song
- Source status text showing the current candidate position

### Notes

- This release is focused on fixing wrong lyric matches without requiring a full re-search
- If the first search result is inaccurate, users can now correct it directly from the menu bar

## v0.1.1

Configuration and quality-of-life release.

### Added

- Persistent config file support
- CLI helper commands for `status`, `config`, and `offset`
- Menu controls for lyric offset adjustments
- Emoji prefix toggle and custom prefix support
- Manual lyric source switching across search candidates
- Homebrew background service support

### Notes

- If launched through `brew services`, quitting from the menu only exits the current process; use `brew services stop akaama/lyrics-display/lyrics-display` to fully stop it
- Menu bar font and text color remain controlled by macOS and are not configurable in this build

## v0.1.0

Initial public release.

### Added

- Real-time Apple Music lyric display in the macOS menu bar
- Apple Music playback polling through AppleScript
- Timed lyric fetching from NetEase Music
- LRC parsing and current-line matching
- In-memory lyric cache by track and artist
- Fallback display for songs without synced lyrics
- `LYRICS_OFFSET_MS` support for manual sync tuning
- `--help` and `--version` command support
- Homebrew Formula for installation

### Notes

- macOS will ask for Automation permission to control `Music` on first run
- Lyric matching quality depends on NetEase Music search results
- Some songs with alternate naming like `Live` or `Remastered` may not match perfectly
