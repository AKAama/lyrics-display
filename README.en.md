[简体中文](./README.md) | **English**

# lyrics-display

`lyrics-display` is a macOS menu bar app that shows the current Apple Music lyric line in real time.

It is written in Go, reads playback state and built-in lyrics from Apple Music through AppleScript, falls back to NetEase Music for timed lyrics when needed, and updates the menu bar every `500ms`.

## Why

`lyrics-display` is built for a simple workflow:

- keep Apple Music playing
- keep the current lyric visible in the menu bar
- avoid opening a separate floating lyric window

It aims to stay lightweight, fast to launch, and easy to install.

## Features

- Real-time Apple Music lyric display in the macOS menu bar
- Prefer Music app built-in lyrics, then NetEase, then LRCLIB
- Timed LRC parsing and current-line matching
- In-memory lyric cache per track
- Fallback to `Track - Artist` when no lyric is found
- Persistent config file support
- Config-file-based emoji and lyric offset tuning
- Manual switching between lyric source candidates
- Native macOS `.app` for drag-and-drop install

## Quick Start

The easiest path is the macOS app:

```bash
make dmg
```

Open `dist/lyrics-display-*.dmg`, drag `lyrics-display.app` into Applications, and launch it.

Homebrew still works:

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
brew services start akaama/lyrics-display/lyrics-display
```

On first launch, macOS may ask for permission to control `Music`. Allow `lyrics-display` under `System Settings -> Privacy & Security -> Automation`.

## Requirements

- macOS
- Apple Music
- Automation permission for controlling `Music`

## Install

### macOS App (recommended)

```bash
make app    # builds dist/lyrics-display.app
make dmg    # also builds a distributable DMG
```

Drag `lyrics-display.app` into Applications and open it. It stays out of the Dock and shows lyrics in the menu bar.

If macOS refuses to open it, right-click the app and choose Open. Current builds are ad-hoc signed, not notarized.

### Homebrew tap

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
brew services start akaama/lyrics-display/lyrics-display
```

If you only want to run it manually for the current terminal session:

```bash
lyrics-display
```

### Build from source

```bash
go build -o lyrics-display .
./lyrics-display
```

## Usage

```bash
lyrics-display
lyrics-display --help
lyrics-display --version
lyrics-display status
lyrics-display config path
brew services start akaama/lyrics-display/lyrics-display
brew services stop akaama/lyrics-display/lyrics-display
```

Default offset is `350ms`.

## First Run And Permissions

The first time you start the app, macOS may ask for permission to control `Music`.

If lyrics do not appear, check:

`System Settings -> Privacy & Security -> Automation`

and allow your terminal or the installed binary to control `Music`.

## Config File

The default config path is:

```bash
~/Library/Application Support/lyrics-display/config.json
```

The config file supports `JSONC`-style comments, including `//` and `/* ... */`.

You can print the exact path with:

```bash
lyrics-display config path
```

Current supported keys:

- `show_emoji`
- `emoji`
- `offset_ms`
- `slot_width`

Create the default config file:

```bash
lyrics-display config init
```

Show the current config:

```bash
lyrics-display config show
```

Default config example:

```jsonc
{
  "show_emoji": true,
  "emoji": "♪",
  "offset_ms": 350, // positive delays lyrics, negative advances them
  "slot_width": 18 // menu bar lyric slot width in CJK characters, 8-40
}
```

Recommended flow:

```bash
1. Click `打开配置文件` from the menu bar menu
2. Edit the config file directly
3. Save it and restart the app or restart the background service
```

If the current lyric match is wrong, use the menu bar action `换下一个歌词源` to cycle through the next NetEase search candidates. If the current source is Music built-in lyrics, the menu shows `改用在线歌词`.

## How It Works

1. Read the current Apple Music track and playback position through AppleScript.
2. Read official Apple Music TTML lyrics from Music.app's local cache first.
3. If that cache misses, read the track's embedded `lyrics` tag.
4. If still unsynced, search NetEase Music and LRCLIB.
5. Parse `TTML` / `LRC` into a timed lyric timeline.
6. Update the current lyric line in the macOS menu bar every `500ms`.

## Homebrew Release Flow

This repository includes a Formula template at `Formula/lyrics-display.rb`.

Before publishing:

1. Create a Git tag such as `v0.1.0`.
2. Push the tag to `https://github.com/AKAama/lyrics-display`.
3. Download the source tarball for that tag from GitHub.
4. Compute its `sha256`.
5. Update `Formula/lyrics-display.rb` with the real tag URL and `sha256`.
6. Users can then run `brew tap AKAama/lyrics-display && brew install lyrics-display`.

The Homebrew tap now lives in a dedicated repository: `AKAama/homebrew-lyrics-display`.

Current approach:

- The source code lives in `AKAama/lyrics-display`
- The Homebrew tap lives in `AKAama/homebrew-lyrics-display`
- Users can install through `brew tap AKAama/lyrics-display && brew install lyrics-display`

This matches the default Homebrew tap naming convention and keeps distribution concerns separate from the main source repository.

## Local Development

```bash
make build
make test
make run
make version
make app
make dmg
```

## Release Notes

- Changelog: `CHANGELOG.md`
- GitHub release draft text: `docs/release-v0.1.0.md`
- Chinese guide: `README.md`

## Notes

- Official Apple Music timed lyrics are not exposed through AppleScript `lyrics`; the app reads Music.app's local TTML cache instead. If the lyrics pane has never been opened for the current song, it falls back to NetEase / LRCLIB.
- Some songs may match imperfectly when titles include `Live`, `Remastered`, or alternate naming.
- Menu bar updates are intentionally conservative to reduce flicker.
- The app is currently ad-hoc signed and not notarized.

## Troubleshooting

- If nothing appears in the menu bar, confirm the app is running and `Music` is open.
- If only song title and artist appear, the current track may not have matched synced lyrics.
- If the lyric feels early or late, adjust `offset_ms` in the config file and restart the app.
- After installing the `.app`, closing the terminal does not quit the menu bar app. Use Quit from the menu.
- If you launch `lyrics-display` directly from a terminal, closing that terminal will also stop the app; install the app or use `brew services start akaama/lyrics-display/lyrics-display` for background use.
- If you started the app through `brew services`, the menu shows `停止后台服务` and unloading the LaunchAgent stops it for good. To start it again, run `brew services start akaama/lyrics-display/lyrics-display`.
- This build does not support custom menu bar font or text color. On macOS, those are controlled by the system menu bar and are not exposed through the current `systray` approach.
