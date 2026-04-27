# lyrics-display v0.1.1

Configuration and usability update for `lyrics-display`.

## Highlights

- Added a persistent config file for saved user preferences
- Added helper commands for status, config inspection, emoji toggle, and lyric offset tuning
- Added menu controls for quicker lyric offset adjustments
- Added Homebrew background service support so the app can keep running after the terminal is closed

## Useful Commands

```bash
lyrics-display status
lyrics-display config show
lyrics-display config path
lyrics-display config set emoji off
lyrics-display config set emoji-char "🎵"
lyrics-display config set offset-ms 250
lyrics-display offset +100
lyrics-display offset -100
brew services start akaama/lyrics-display/lyrics-display
brew services stop akaama/lyrics-display/lyrics-display
```

## Notes

- If you launched the app with `brew services`, quitting from the menu only exits the current process; Homebrew may start it again automatically
- To fully stop the app, use `brew services stop akaama/lyrics-display/lyrics-display`
- Menu bar font and text color are still controlled by macOS and are not configurable in this release
