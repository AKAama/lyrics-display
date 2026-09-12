# lyrics-display v0.2.1

Patch release focused on playback compatibility, lyric source switching, and menu bar sizing.

## Highlights

- Accept AppleScript playback positions formatted with a comma decimal separator
- Continue through alternative lyric candidates when an online source fails
- Add detailed logs for source switching and lyric reloads
- Apply `slot_width` consistently to both scrolling text and the native menu bar item

## Notes

- Current builds are ad-hoc signed, not notarized; first open may need right-click -> Open
- Homebrew Formula metadata must be updated after the `v0.2.1` tag is available so its source archive SHA-256 can be calculated