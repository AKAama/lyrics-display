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
- 支持持久化配置文件
- 支持命令行和菜单内调整歌词同步偏移
- 支持 Emoji 前缀开关和自定义符号
- 支持在候选歌词源之间手动切换

## 快速开始

```bash
brew tap AKAama/lyrics-display
brew install lyrics-display
brew services start akaama/lyrics-display/lyrics-display
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
brew services start akaama/lyrics-display/lyrics-display
```

### 方式二：源码编译

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
lyrics-display config show
lyrics-display config path
lyrics-display config set emoji off
lyrics-display config set emoji-char "🎵"
lyrics-display config set offset-ms 450
lyrics-display offset +100
lyrics-display offset -100
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

你也可以通过下面的命令查看真实路径：

```bash
lyrics-display config path
```

当前支持的配置项：

- `show_emoji`
- `emoji`
- `offset_ms`

可以先生成一份默认配置：

```bash
lyrics-display config init
```

查看当前配置：

```bash
lyrics-display config show
```

快速修改示例：

```bash
lyrics-display config set emoji off
lyrics-display config set emoji-char "🎵"
lyrics-display config set offset-ms 250
```

或者直接做偏移微调：

```bash
lyrics-display offset +100
lyrics-display offset -100
```

如果当前歌词匹配不准，可以直接在菜单栏菜单里点击 `换下一个歌词源`，程序会按候选顺序切到下一条网易云搜索结果。

## 工作原理

1. 通过 AppleScript 读取 Apple Music 当前歌曲和播放进度。
2. 在网易云音乐中搜索最合适的歌词结果。
3. 将返回的 `LRC` 解析为时间轴结构。
4. 以 `500ms` 的频率刷新菜单栏中的当前歌词。

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
- 如果你直接在终端里运行 `lyrics-display`，关闭终端后程序也会退出；长期使用请改用 `brew services start akaama/lyrics-display/lyrics-display`。
- 如果你是通过 `brew services` 启动的，菜单里的“退出”只会结束当前进程，服务管理器可能会自动再次拉起它。真正停用请执行 `brew services stop akaama/lyrics-display/lyrics-display`，恢复运行请执行 `brew services start akaama/lyrics-display/lyrics-display`。
- 当前这个版本不能自定义菜单栏文字颜色或字体。这不是缺少配置项，而是因为 macOS 菜单栏文字样式由系统控制，`systray` 路线本身不开放这类定制。
