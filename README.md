**简体中文** | [English](./README.en.md)

# lyrics-display

`lyrics-display` 是一个运行在 macOS 菜单栏上的 Apple Music 实时歌词工具。

它使用 Go 编写，通过 AppleScript 读取 Apple Music 当前播放信息，从网易云音乐拉取带时间轴的歌词，并以 `500ms` 频率刷新菜单栏显示。

## 适合什么场景

这个项目适合下面这种使用方式：

- 一边听 Apple Music
- 一边希望当前歌词始终停留在菜单栏
- 不想额外开一个悬浮歌词窗口

它的目标是轻量、启动快、安装简单。

## 功能特性

- 在 macOS 菜单栏实时显示当前歌词
- 自动匹配歌曲并解析 `LRC` 时间轴
- 按歌曲缓存歌词，减少重复请求
- 没有同步歌词时自动回退为 `歌曲名 - 歌手`
- 支持通过 `LYRICS_OFFSET_MS` 手动微调歌词同步偏移

## 快速开始

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
lyrics-display
```

首次启动时，macOS 可能会请求自动化权限。

## 运行要求

- macOS
- Apple Music
- 允许程序控制 `Music` 的自动化权限

## 安装

### 方式一：Homebrew

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
lyrics-display
```

### 方式二：源码编译

```bash
go build -o lyrics-display .
./lyrics-display
```

## 使用方式

```bash
lyrics-display
lyrics-display --help
lyrics-display --version
```

如需手动调整歌词时间偏移：

```bash
LYRICS_OFFSET_MS=450 lyrics-display
```

默认偏移为 `350ms`。

## 首次运行与权限

第一次启动时，macOS 可能会弹出权限提示，要求允许程序控制 `Music`。

如果没有正常显示歌词，请检查：

`系统设置 -> 隐私与安全性 -> 自动化`

并允许终端或安装后的可执行程序控制 `Music`。

## 工作原理

1. 通过 AppleScript 读取 Apple Music 当前歌曲和播放进度。
2. 在网易云音乐中搜索最合适的歌词结果。
3. 将返回的 `LRC` 解析为时间轴结构。
4. 以 `500ms` 的频率刷新菜单栏中的当前歌词。

## Homebrew 说明

本项目当前采用“源码仓库同时兼作 tap 仓库”的方式。

也就是说：

- 代码仓库就是 `AKAama/lyrics-display`
- Homebrew Formula 直接放在仓库里的 `Formula/lyrics-display.rb`
- 用户安装时执行 `brew tap AKAama/lyrics-display`

这种方式的优点是：

- 第一个版本上线最快
- 不需要额外维护一个 `homebrew-lyrics-display` 仓库
- 适合个人项目或早期 MVP

需要注意的是，Homebrew 的 tap 通常更常见的命名方式是单独建一个仓库，例如：

`homebrew-lyrics-display`

如果以后你希望这条分发链路更标准，可以演进成：

1. 主仓库继续放源码：`AKAama/lyrics-display`
2. 新建 tap 仓库：`AKAama/homebrew-lyrics-display`
3. 只把 Formula 放到 tap 仓库里

这样用户安装时会变成：

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
```

对用户来说命令几乎不变，但仓库职责会更清晰。

## 发版流程

当前 Homebrew Formula 跟随 GitHub tag 发布。

一次完整发布大致是：

1. 提交代码到 `main`
2. 打标签，例如 `v0.1.1`
3. 推送 tag 到 GitHub
4. 下载该 tag 的源码压缩包
5. 计算 `sha256`
6. 更新 `Formula/lyrics-display.rb` 中的 `url` 和 `sha256`
7. 提交并推送 Formula 更新
8. 在 GitHub 创建 Release

## 相关文档

- 英文说明：`README.en.md`
- 更新记录：`CHANGELOG.md`
- `v0.1.0` 发布文案：`docs/release-v0.1.0.md`

## 已知限制

- 当前歌词源依赖网易云音乐搜索与歌词接口
- `Live`、`Remastered`、地区版命名等情况可能影响匹配准确率
- 当前版本只做了内存缓存，重启后不会保留歌词缓存

## 常见问题

- 如果菜单栏没有显示内容，先确认程序已经启动且 `Music` 已打开。
- 如果只显示歌曲名和歌手，通常表示当前歌曲没有匹配到同步歌词。
- 如果歌词快了或慢了，可以调整 `LYRICS_OFFSET_MS` 后重新启动程序。
