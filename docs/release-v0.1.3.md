# lyrics-display v0.1.3

Release focused on simplifying the user-facing settings workflow.

## Highlights

- Removed direct emoji and lyric offset toggles from the menu bar
- Added a menu action to reveal the config file in Finder
- Kept advanced customization available through the config file

## Why This Release

The menu bar should stay focused on everyday actions like checking status and switching lyric sources.

Advanced customization such as emoji style and lyric offset is still useful, but it is better handled in a single persistent config file instead of being spread across many menu actions.

## Notes

- The menu is now cleaner and less overwhelming
- Use the `打开配置文件` action from the menu to jump straight to the config file
- Emoji and offset support still exist; the preferred way to change them is by editing the config file
