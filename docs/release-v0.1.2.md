# lyrics-display v0.1.2

Release focused on fixing lyric mismatch issues more gracefully.

## Highlights

- Added a menu action to switch to the next lyric source candidate
- Preserves multiple NetEase search candidates for the current song
- Shows the current candidate position in the lyric source status

## Why This Release

Sometimes the top lyric search result is not the correct one.

With `v0.1.2`, users no longer need to wait for a code change or tweak the search logic manually. They can simply click `换下一个歌词源` from the menu bar and move through the next available candidates.

## Notes

- This improves recovery from wrong lyric matches, but search quality still depends on NetEase Music results
- Menu bar font and text color are still controlled by macOS and are not configurable in this release
