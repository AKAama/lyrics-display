# lyrics-display v0.1.0

First public release of `lyrics-display`.

## Highlights

- Shows the current Apple Music lyric line directly in the macOS menu bar
- Fetches timed lyrics automatically and updates every `500ms`
- Falls back to `Track - Artist` when synced lyrics are unavailable
- Supports manual sync tuning with `LYRICS_OFFSET_MS`
- Installable through Homebrew

## Install

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
lyrics-display
```

## First Run

On first launch, macOS may ask for permission to control `Music`.

If lyrics do not appear, go to:

`System Settings -> Privacy & Security -> Automation`

and allow the terminal or installed binary to control `Music`.

## Known Limitations

- Lyric fetching currently depends on NetEase Music search and lyric endpoints
- Song title variants such as `Live`, `Remastered`, or regional naming may affect lyric matching
- This release uses in-memory caching only
