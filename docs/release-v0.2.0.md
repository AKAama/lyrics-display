# lyrics-display v0.2.0

Release focused on using Apple Music's own lyrics, making install easier, and keeping the menu bar slot stable.

## Highlights

- Read official Apple Music timed lyrics from Music.app's local TTML cache
- Fall back to LRCLIB when NetEase is blocked
- Ship a native `.app` / DMG for drag-and-drop install
- Keep the menu bar lyric slot at a fixed pixel width
- Make slot width configurable with `slot_width`
- Stop Homebrew services cleanly from the menu bar

## Why This Release

Apple Music's on-screen lyrics are not available through AppleScript `lyrics of current track`. This release reads the same TTML cache Music.app already downloaded, so catalog songs can show official timed lyrics without going through NetEase first.

Install should also feel like a Mac app: drag `lyrics-display.app` into Applications. Homebrew remains supported, and quitting from the menu now unloads the LaunchAgent instead of being restarted by `keep_alive`.

## Notes

- If Apple Music lyrics miss, open the lyrics pane once in Music so the TTML cache is populated, then use `重新获取歌词`
- `slot_width` is measured in CJK character widths, default `18`, range `8-40`
- Current builds are ad-hoc signed, not notarized; first open may need right-click → Open
- After upgrading Homebrew, restart with `brew services restart akaama/lyrics-display/lyrics-display`
