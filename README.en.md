[简体中文](./README.md) | **English**

# lyrics-display

`lyrics-display` is a macOS menu bar app that shows the current Apple Music lyric line in real time.

It is written in Go, reads playback state from Apple Music through AppleScript, fetches timed lyrics from NetEase Music, and updates the menu bar every `500ms`.

## Why

`lyrics-display` is built for a simple workflow:

- keep Apple Music playing
- keep the current lyric visible in the menu bar
- avoid opening a separate floating lyric window

It aims to stay lightweight, fast to launch, and easy to install.

## Features

- Real-time Apple Music lyric display in the macOS menu bar
- Timed LRC parsing and current-line matching
- In-memory lyric cache per track
- Fallback to `Track - Artist` when no lyric is found
- Adjustable lyric sync offset through `LYRICS_OFFSET_MS`

## Quick Start

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
brew services start lyrics-display
```

On first launch, macOS may ask for permission to control `Music`.

## Requirements

- macOS
- Apple Music
- Automation permission for controlling `Music`

## Install

### Homebrew tap

After you create a release and update the Formula checksum:

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
```

Then start it with:

```bash
brew services start lyrics-display
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
brew services start lyrics-display
brew services stop lyrics-display
```

Optional environment variable:

```bash
LYRICS_OFFSET_MS=450 lyrics-display
```

Default offset is `350ms`.

## First Run And Permissions

The first time you start the app, macOS may ask for permission to control `Music`.

If lyrics do not appear, check:

`System Settings -> Privacy & Security -> Automation`

and allow your terminal or the installed binary to control `Music`.

## How It Works

1. Read the current Apple Music track and playback position through AppleScript.
2. Search NetEase Music for the best lyric match.
3. Parse the returned `LRC` data into a timed lyric timeline.
4. Update the current lyric line in the macOS menu bar every `500ms`.

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
make run
make version
```

## Release Notes

- Changelog: `CHANGELOG.md`
- GitHub release draft text: `docs/release-v0.1.0.md`
- Chinese guide: `README.md`

## Notes

- The lyric source depends on NetEase Music search and lyric endpoints.
- Some songs may match imperfectly when titles include `Live`, `Remastered`, or alternate naming.
- Menu bar updates are intentionally conservative to reduce flicker.

## Troubleshooting

- If nothing appears in the menu bar, confirm the app is running and `Music` is open.
- If only song title and artist appear, the current track may not have matched synced lyrics.
- If the lyric feels early or late, adjust `LYRICS_OFFSET_MS` and restart the app.
- If you launch `lyrics-display` directly from a terminal, closing that terminal will also stop the app; use `brew services start lyrics-display` for background use.
