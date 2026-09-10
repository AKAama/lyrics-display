package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestResolveLyricsPrefersTimedBuiltin(t *testing.T) {
	builtin := "[00:01.00]内置第一句\n[00:05.00]内置第二句"
	online := lyricDocument{
		Lines:      []lyricLine{{At: time.Second, Text: "在线歌词"}},
		SourceKind: lyricSourceNetease,
		SourceID:   42,
	}

	doc := resolveLyrics("Song", "Artist", builtin, online, true)
	if doc.SourceKind != lyricSourceMusic {
		t.Fatalf("SourceKind = %q, want music", doc.SourceKind)
	}
	if len(doc.Lines) != 2 {
		t.Fatalf("len(Lines) = %d, want 2", len(doc.Lines))
	}
	if doc.Lines[0].Text != "内置第一句" {
		t.Fatalf("first line = %q", doc.Lines[0].Text)
	}
	if !doc.HadBuiltin {
		t.Fatal("expected HadBuiltin")
	}
}

func TestResolveLyricsFallsBackToOnlineWhenBuiltinHasNoTimeline(t *testing.T) {
	builtin := "作词：测试\n这是没有时间轴的歌词"
	online := lyricDocument{
		Lines:      []lyricLine{{At: time.Second, Text: "在线同步歌词"}},
		SourceKind: lyricSourceNetease,
		SourceID:   7,
	}

	doc := resolveLyrics("Song", "Artist", builtin, online, true)
	if doc.SourceKind != lyricSourceNetease {
		t.Fatalf("SourceKind = %q, want netease", doc.SourceKind)
	}
	if doc.Lines[0].Text != "在线同步歌词" {
		t.Fatalf("line = %q", doc.Lines[0].Text)
	}
}

func TestResolveLyricsUsesPlainBuiltinWhenOnlineMissing(t *testing.T) {
	doc := resolveLyrics("Song", "Artist", "作词：甲\n作曲：乙\n第一句歌词", lyricDocument{}, false)
	if doc.SourceKind != lyricSourceMusic {
		t.Fatalf("SourceKind = %q, want music", doc.SourceKind)
	}
	if !doc.Untimed {
		t.Fatal("expected untimed builtin lyrics")
	}
	if doc.Lines[0].Text != "第一句歌词" {
		t.Fatalf("plain line = %q", doc.Lines[0].Text)
	}
}

func TestResolveLyricsFallbackWhenNothingMatches(t *testing.T) {
	doc := resolveLyrics("Night", "Fish", "", lyricDocument{}, false)
	if doc.SourceKind != lyricSourceNone {
		t.Fatalf("SourceKind = %q, want empty", doc.SourceKind)
	}
	if doc.DisplayName != "Night - Fish" {
		t.Fatalf("DisplayName = %q", doc.DisplayName)
	}
}

func TestParseLRCAcceptsCRLF(t *testing.T) {
	lines := parseLRC("[00:01.00]hello\r\n[00:02.50]world\r")
	if len(lines) != 2 {
		t.Fatalf("len = %d, want 2", len(lines))
	}
	if lines[1].At != 2500*time.Millisecond {
		t.Fatalf("second timestamp = %s", lines[1].At)
	}
}

func TestFirstPlainLyricLineSkipsMetadata(t *testing.T) {
	raw := "[ti:Song]\n作词：甲\nComposer: 乙\n\n真正的歌词"
	got := firstPlainLyricLine(raw)
	if got != "真正的歌词" {
		t.Fatalf("got %q", got)
	}
}

func TestPreserveAndRestoreBuiltin(t *testing.T) {
	builtin := lyricDocument{
		Track:      "Song",
		Artist:     "Artist",
		Lines:      []lyricLine{{At: time.Second, Text: "内置"}},
		SourceKind: lyricSourceMusic,
		HadBuiltin: true,
	}
	online := lyricDocument{
		Track:       "Song",
		Artist:      "Artist",
		Lines:       []lyricLine{{At: 2 * time.Second, Text: "在线"}},
		SourceKind:  lyricSourceNetease,
		Candidates:  []lyricCandidate{{ID: 1, Name: "Song", Artist: "Artist"}},
		SourceIndex: 0,
	}

	merged := preserveBuiltin(online, builtin)
	if !merged.HadBuiltin || len(merged.BuiltinLines) != 1 {
		t.Fatalf("preserveBuiltin failed: %+v", merged)
	}
	if merged.SourceKind != lyricSourceNetease {
		t.Fatalf("merged source = %q", merged.SourceKind)
	}

	restored := restoreBuiltin(merged)
	if restored.SourceKind != lyricSourceMusic {
		t.Fatalf("restored source = %q", restored.SourceKind)
	}
	if restored.Lines[0].Text != "内置" {
		t.Fatalf("restored line = %q", restored.Lines[0].Text)
	}
}

func TestIsLyricMetadataLine(t *testing.T) {
	if !isLyricMetadataLine("作词：周杰伦") {
		t.Fatal("expected metadata")
	}
	if isLyricMetadataLine("不能说的秘密") {
		t.Fatal("lyric line should not be metadata")
	}
	if !isLyricMetadataLine("[ar:Artist]") {
		t.Fatal("expected [ar:] metadata")
	}
}

func TestCurrentLyric(t *testing.T) {
	lines := []lyricLine{
		{At: time.Second, Text: "one"},
		{At: 3 * time.Second, Text: "two"},
	}
	if got := currentLyric(lines, 500*time.Millisecond); got != "" {
		t.Fatalf("before first line: %q", got)
	}
	if got := currentLyric(lines, 2*time.Second); got != "one" {
		t.Fatalf("got %q want one", got)
	}
	if got := currentLyric(lines, 4*time.Second); got != "two" {
		t.Fatalf("got %q want two", got)
	}
}

func TestMergeOnlinePrefersNeteaseThenLRCLIB(t *testing.T) {
	netease := lyricDocument{
		Lines:       []lyricLine{{At: time.Second, Text: "网易云"}},
		SourceKind:  lyricSourceNetease,
		SourceIndex: 0,
		Candidates:  []lyricCandidate{{ID: 1, Name: "A", Kind: lyricSourceNetease}},
	}
	lrclib := lyricDocument{
		Lines:       []lyricLine{{At: time.Second, Text: "LRCLIB"}},
		SourceKind:  lyricSourceLRCLIB,
		SourceIndex: 0,
		Candidates:  []lyricCandidate{{ID: 2, Name: "B", Kind: lyricSourceLRCLIB}},
	}

	doc, err := mergeOnlineDocs(netease, nil, lrclib, nil)
	if err != nil {
		t.Fatal(err)
	}
	if doc.SourceKind != lyricSourceNetease || doc.Lines[0].Text != "网易云" {
		t.Fatalf("expected netease win, got %+v", doc)
	}
	if len(doc.Candidates) != 2 {
		t.Fatalf("merged candidates = %d", len(doc.Candidates))
	}

	doc, err = mergeOnlineDocs(lyricDocument{}, fmt.Errorf("eof"), lrclib, nil)
	if err != nil {
		t.Fatal(err)
	}
	if doc.SourceKind != lyricSourceLRCLIB {
		t.Fatalf("expected lrclib fallback, got %q", doc.SourceKind)
	}
	if doc.SourceIndex != 0 {
		t.Fatalf("SourceIndex = %d, want 0 when netease has no candidates", doc.SourceIndex)
	}

	_, err = mergeOnlineDocs(lyricDocument{}, fmt.Errorf("eof"), lyricDocument{}, fmt.Errorf("no songs"))
	if err == nil {
		t.Fatal("expected error when both sources fail")
	}
}

func TestDocumentFromCandidateParsesSyncedLRC(t *testing.T) {
	candidates := []lyricCandidate{{
		ID:     9,
		Name:   "说好的幸福呢",
		Artist: "周杰伦",
		Kind:   lyricSourceLRCLIB,
		Synced: "[00:01.00]你的回话凌乱着\n[00:05.00]在这个时刻",
	}}
	doc, err := documentFromCandidate("说好的幸福呢", "周杰伦", candidates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if doc.SourceKind != lyricSourceLRCLIB {
		t.Fatalf("kind = %q", doc.SourceKind)
	}
	if got := currentLyric(doc.Lines, 3*time.Second); got != "你的回话凌乱着" {
		t.Fatalf("got %q", got)
	}
}

func TestTrimForMenuBar(t *testing.T) {
	long := strings.Repeat("字", menuBarMaxRunes+5)
	got := trimForMenuBar(long)
	if []rune(got)[len([]rune(got))-1] != '…' {
		t.Fatalf("expected ellipsis, got %q", got)
	}
	if len([]rune(got)) != menuBarMaxRunes {
		t.Fatalf("len = %d", len([]rune(got)))
	}
}
