**简体中文** | [English](./README.en.md)

# lyrics-display

`lyrics-display` 是一个运行在 macOS 菜单栏上的 Apple Music 实时歌词工具。

它使用 Go 编写，通过 AppleScript 读取 Apple Music 当前播放信息和内置歌词，必要时再从网易云音乐拉取带时间轴的歌词，并以 `500ms` 频率刷新菜单栏显示。

## 适合什么场景

这个项目适合下面这种使用方式：

- 一边听 Apple Music
- 一边希望当前歌词始终停留在菜单栏
- 不想额外开一个悬浮歌词窗口

它的目标是轻量、启动快、安装简单。

## 功能特性

- 在 macOS 菜单栏实时显示当前歌词
- 优先读取「音乐」App 内置歌词，没有时间轴时再匹配网易云，网易云不可用时回退到 LRCLIB
- 自动匹配歌曲并解析 `LRC` 时间轴
- 按歌曲缓存歌词，减少重复请求
- 没有同步歌词时自动回退为 `歌曲名 - 歌手`
- 支持持久化配置文件
- 支持通过配置文件调整 Emoji 和歌词偏移
- 支持在候选歌词源之间手动切换
- 提供可拖拽安装的 macOS `.app`

## 快速开始

最省事的方式是安装 App：

```bash
make dmg
```

然后打开 `dist/lyrics-display-*.dmg`，把 `lyrics-display.app` 拖进「应用程序」，双击启动。

也可以继续用 Homebrew：

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
brew services start akaama/lyrics-display/lyrics-display
```

首次启动时，macOS 可能会请求自动化权限。请在 `系统设置 -> 隐私与安全性 -> 自动化` 中允许 `lyrics-display` 控制 `Music`。

## 运行要求

- macOS
- Apple Music
- 允许程序控制 `Music` 的自动化权限

## 安装

### 方式一：macOS App（推荐）

从源码打包：

```bash
make app    # 生成 dist/lyrics-display.app
make dmg    # 再生成可分发的 DMG
```

把 `lyrics-display.app` 拖到「应用程序」后打开即可。它不会出现在 Dock 里，歌词直接显示在菜单栏。

如果系统提示无法打开，请右键 App → 打开。当前构建是 ad-hoc 签名，尚未公证。

### 方式二：Homebrew

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
brew services start akaama/lyrics-display/lyrics-display
```

### 方式三：源码编译

```bash
go build -o lyrics-display .
./lyrics-display
```

如果你只是临时手动运行，也可以直接执行：

```bash
lyrics-display
```

## 使用方式

```bash
lyrics-display
lyrics-display --help
lyrics-display --version
lyrics-display status
lyrics-display config path
brew services start akaama/lyrics-display/lyrics-display
brew services stop akaama/lyrics-display/lyrics-display
```

默认偏移为 `350ms`。

## 首次运行与权限

第一次启动时，macOS 可能会弹出权限提示，要求允许程序控制 `Music`。

如果没有正常显示歌词，请检查：

`系统设置 -> 隐私与安全性 -> 自动化`

并允许终端或安装后的可执行程序控制 `Music`。

## 配置文件

配置文件默认位于：

```bash
~/Library/Application Support/lyrics-display/config.json
```

配置文件支持 `JSONC` 风格注释，也就是可以写 `//` 和 `/* ... */` 注释。

你也可以通过下面的命令查看真实路径：

```bash
lyrics-display config path
```

当前支持的配置项：

- `show_emoji`
- `emoji`
- `offset_ms`
- `slot_width`

可以先生成一份默认配置：

```bash
lyrics-display config init
```

查看当前配置：

```bash
lyrics-display config show
```

默认配置大致如下：

```jsonc
{
  "show_emoji": true,
  "emoji": "♪",
  "offset_ms": 350, // 正数表示延后，负数表示提前
  "slot_width": 18 // 菜单栏歌词槽宽，按汉字宽度计，范围 8-40
}
```

推荐做法：

```bash
1. 在菜单栏菜单中点击 `打开配置文件`
2. 直接编辑配置文件
3. 保存后重启程序或重启后台服务
```

如果当前歌词匹配不准，可以直接在菜单栏菜单里点击 `换下一个歌词源`，程序会按候选顺序切到下一条网易云搜索结果。如果当前用的是 Music 内置歌词，菜单会显示 `改用在线歌词`。

## 工作原理

1. 通过 AppleScript 读取 Apple Music 当前歌曲和播放进度。
2. 先读取 Apple Music 官方 TTML 歌词缓存（和「音乐」App 歌词面板同一份）。
3. 没有官方缓存时，再读曲目内嵌的 `lyrics` 标签。
4. 仍没有时间轴时，并行搜索网易云音乐和 LRCLIB。
5. 将 `TTML` / `LRC` 解析为时间轴结构。
6. 以 `500ms` 的频率刷新菜单栏中的当前歌词。

## Homebrew 说明

本项目当前使用独立的 Homebrew tap 仓库。

也就是说：

- 源码仓库是 `AKAama/lyrics-display`
- tap 仓库是 `AKAama/homebrew-lyrics-display`
- Homebrew Formula 由 tap 仓库提供

这种方式的优点是：

- 符合 Homebrew 的默认命名约定
- 用户可以直接使用标准的 `brew tap` 命令
- 源码仓库和分发仓库职责更清晰

当前结构就是：

1. 主仓库放源码：`AKAama/lyrics-display`
2. tap 仓库放 Formula：`AKAama/homebrew-lyrics-display`

这样用户安装时会变成：

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
```

这样用户体验和常见的第三方 Homebrew 包一致。

## 发版流程

当前 Homebrew Formula 跟随 GitHub tag 发布。

一次完整发布大致是：

1. 提交代码到 `main`
2. 打标签，例如 `v0.2.0`
3. 推送 tag 到 GitHub
4. 运行 `VERSION=0.2.0 make dmg`，把 DMG 挂到 GitHub Release
5. 下载该 tag 的源码压缩包
6. 计算 `sha256`
7. 更新 `Formula/lyrics-display.rb` 中的 `url` 和 `sha256`，并同步 tap 仓库里的 `service` 启动参数 `--service`
8. 提交并推送 Formula 更新
9. 在 GitHub 创建 Release

## 相关文档

- 英文说明：`README.en.md`
- 更新记录：`CHANGELOG.md`
- `v0.1.0` 发布文案：`docs/release-v0.1.0.md`

## 已知限制

- Apple Music 界面上的官方歌词不在 AppleScript 的 `lyrics` 字段里；程序改为读取「音乐」App 本地 TTML 缓存。若从未打开过歌词面板，缓存可能还没有当前歌，这时会回退到网易云 / LRCLIB
- `Live`、`Remastered`、地区版命名等情况可能影响在线匹配准确率
- 当前版本只做了内存缓存，重启后不会保留歌词缓存
- App 目前是 ad-hoc 签名，未做 Apple 公证

## 常见问题

- 如果菜单栏没有显示内容，先确认程序已经启动且 `Music` 已打开。
- 如果只显示歌曲名和歌手，通常表示当前歌曲没有匹配到同步歌词。
- 如果歌词快了或慢了，可以调整配置文件里的 `offset_ms` 后重新启动程序。
- 安装成 `.app` 后，关闭终端不会影响菜单栏歌词；从菜单里点「退出」即可结束。
- 如果你直接在终端里运行 `lyrics-display`，关闭终端后程序也会退出。长期使用请安装 App，或执行 `brew services start akaama/lyrics-display/lyrics-display`。
- 如果你是通过 `brew services` 启动的，菜单会显示「停止后台服务」，点它会卸载 LaunchAgent，不会再被自动拉起。恢复运行请执行 `brew services start akaama/lyrics-display/lyrics-display`。
- 当前这个版本不能自定义菜单栏文字颜色或字体。这不是缺少配置项，而是因为 macOS 菜单栏文字样式由系统控制，`systray` 路线本身不开放这类定制。
